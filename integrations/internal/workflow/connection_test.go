package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"integrations/internal/activity"
)

// The tests drive ConnectionWorkflow under the Temporal test env with the activities
// mocked by name (env.OnActivity). They assert the orchestration — StartRun →
// DiscoverUnits → the continuous worker-pool drain → SettleConnection, the re-launch
// of a not-done unit, cooperative stop, the continueAsNew bound, and that a slow
// straggler does NOT collapse pool concurrency to 1 — without a real Temporal or DB.

const (
	testConnID  = "conn-1"
	testTenant  = "tenant-1"
	testGitHub  = "github"
	disconnect  = SignalDisconnect
	triggerSync = SignalTriggerSync
	stopSync    = SignalStopSync
)

func testInput() ConnectionWorkflowInput {
	return ConnectionWorkflowInput{ConnectionID: testConnID, TenantID: testTenant, ConnectorID: testGitHub}
}

// mockControlActivities stubs the cheap control-plane activities with happy-path
// returns. The drain (NextUnitBatch/SyncUnit) is set per-test.
func mockControlActivities(env *testsuite.TestWorkflowEnvironment) {
	a := &activity.Activities{}
	env.OnActivity(a.StartRun, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(a.DiscoverUnits, mock.Anything, mock.Anything).
		Return(activity.DiscoverResult{Count: 0}, nil)
	env.OnActivity(a.SettleConnection, mock.Anything, mock.Anything).
		Return(activity.SettleResult{Status: "connected"}, nil)
	mockSchedule(env, true, defaultSyncIntervalSeconds)
	mockCapacity(env, 4)
}

// mockSchedule stubs the GetSyncSchedule control activity the idle wait reads on every
// entry — auto-sync on with the default cadence, so the periodic timer is armed as today.
// Maybe (not exactly-once): the run-driven tests may settle and end before re-parking.
func mockSchedule(env *testsuite.TestWorkflowEnvironment, enabled bool, intervalSeconds int32) {
	a := &activity.Activities{}
	env.OnActivity(a.GetSyncSchedule, mock.Anything, mock.Anything).
		Return(activity.ScheduleResult{Enabled: enabled, IntervalSeconds: intervalSeconds}, nil).Maybe()
}

// mockCapacity stubs the admission-controller macro to a fixed batch size P (ADR-0036),
// so the orchestration tests drive a deterministic fan-out without a real limiter/DB.
func mockCapacity(env *testsuite.TestWorkflowEnvironment, p int) {
	a := &activity.Activities{}
	env.OnActivity(a.Capacity, mock.Anything, mock.Anything).
		Return(activity.CapacityResult{P: p}, nil)
}

// poolBatchSource models the sync_unit table for the pool's filtered NextUnitBatch.
// The pool launches a unit, holds it in its in-flight set until the future settles,
// and asks NextUnitBatch (filtered in the workflow) for fresh ids each refill. We
// model the table as a fixed set of pending ids and let SyncUnit retire them: a
// SyncUnit that reports Done removes the id; NextUnitBatch returns the remaining
// pending ids (the workflow itself filters out the ones still in flight). This is the
// same contract the real SQL has — return everything still 'syncing'/'pending'.
type poolBatchSource struct {
	mu      sync.Mutex
	pending map[string]bool
}

func newPoolBatchSource(ids ...string) *poolBatchSource {
	s := &poolBatchSource{pending: make(map[string]bool, len(ids))}
	for _, id := range ids {
		s.pending[id] = true
	}
	return s
}

func (s *poolBatchSource) batch(_ context.Context, in activity.BatchInput) (activity.BatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id := range s.pending {
		ids = append(ids, id)
	}
	// Stable order so the fan-out is deterministic; the real SQL ORDERs BY external_id.
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] < ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	if in.Limit > 0 && len(ids) > in.Limit {
		ids = ids[:in.Limit]
	}
	return activity.BatchResult{UnitIDs: ids}, nil
}

func (s *poolBatchSource) retire(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, id)
}

// TestTriggerSyncDrivesRun: a TriggerSync drives StartRun → DiscoverUnits → the pool
// drain (three units retired) → SettleConnection. A Disconnect after the run ends the
// workflow so the test terminates. The pool keeps up to P=4 in flight; with three
// units each completing Done, all three are synced exactly once.
func TestTriggerSyncDrivesRun(t *testing.T) {
	s := &testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	a := &activity.Activities{}

	mockControlActivities(env)

	src := newPoolBatchSource("r1", "r2", "r3")
	env.OnActivity(a.NextUnitBatch, mock.Anything, mock.Anything).Return(src.batch)

	synced := 0
	env.OnActivity(a.SyncUnit, mock.Anything, mock.Anything).
		Return(func(_ context.Context, in activity.SyncInput) (activity.SyncResult, error) {
			synced++
			src.retire(in.UnitID)
			return activity.SyncResult{Done: true, Status: "synced"}, nil
		})

	// Fire the trigger to start the run, then disconnect to end the workflow.
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(triggerSync, nil) }, 0)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(disconnect, nil) }, time.Second)

	env.ExecuteWorkflow(ConnectionWorkflow, testInput())

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 3, synced, "all three units were synced exactly once")
	env.AssertExpectations(t)
}

// TestNotDoneUnitReinvoked: a unit that returns Done=false (a chunk-budget stop) is
// re-launched on a later refill (it is out of the in-flight set, still 'syncing', so
// NextUnitBatch re-returns it and the pool picks it up again). We model the table by
// returning the same unit until SyncUnit reports done — and assert it was invoked
// exactly twice (one chunk that did not finish, then the chunk that did).
func TestNotDoneUnitReinvoked(t *testing.T) {
	s := &testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	a := &activity.Activities{}
	mockControlActivities(env)

	// The unit stays pending (NextUnitBatch keeps returning it) until SyncUnit retires
	// it on its second invocation. The pool must NOT double-launch it while in flight.
	src := newPoolBatchSource("big")
	env.OnActivity(a.NextUnitBatch, mock.Anything, mock.Anything).Return(src.batch)

	calls := 0
	env.OnActivity(a.SyncUnit, mock.Anything, mock.Anything).
		Return(func(_ context.Context, in activity.SyncInput) (activity.SyncResult, error) {
			calls++
			if calls >= 2 {
				src.retire(in.UnitID) // second chunk caught up → terminal
				return activity.SyncResult{Done: true, Status: "synced"}, nil
			}
			return activity.SyncResult{Done: false, Status: "syncing"}, nil // budget-stopped
		})

	env.RegisterDelayedCallback(func() { env.SignalWorkflow(triggerSync, nil) }, 0)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(disconnect, nil) }, time.Second)

	env.ExecuteWorkflow(ConnectionWorkflow, testInput())

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 2, calls, "the not-done unit was re-launched once (chunked resume)")
	env.AssertExpectations(t)
}

// TestStragglerDoesNotBlockPool is the regression test for the throughput bug: a
// single slow unit must NOT block the other slots. This is the live failure mode — one
// 958-issue repo (the "aaa-straggler") that takes minutes while many small repos wait
// behind it. Under the OLD batch-barrier the drain fanned out a slice then awaited the
// WHOLE slice (f.Get on every future) before launching anything new — so a unit that
// does not return holds the slice, every other slot sits idle, and effective
// concurrency collapses to 1 (the live symptom: in_flight=1 with 5 units idle).
//
// The proof is structural and deterministic (no reliance on wall-clock parallelism):
// the straggler BLOCKS until the drain is cancelled (it returns only when stopped), so
// it occupies exactly one slot for the whole run. We queue MORE fast units than the
// pool's spare capacity (8 fast units, P-1 = 3 free slots), so draining them all
// REQUIRES the freed slots to refill repeatedly while the straggler is still in flight.
//   - Pool: straggler holds 1 slot; the other 3 cycle and retire all 8 fast units; the
//     stop then releases the straggler and the run settles. fastSynced == 8.
//   - Barrier (the bug): the first slice [straggler, fast-1..fast-3] wedges on the
//     straggler's f.Get; no further slice launches, so at MOST 3 fast units run before
//     the stop unwedges it. fastSynced would be 3, never 8 — this test fails on the bug.
func TestStragglerDoesNotBlockPool(t *testing.T) {
	s := &testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	a := &activity.Activities{}
	mockControlActivities(env) // P = 4

	// One blocking straggler plus 8 fast units — more fast units than the pool's spare
	// capacity (P-1 = 3), so draining them all proves the freed slots refilled while the
	// straggler still held its slot. "aaa-" sorts the straggler first so it is launched
	// in the very first refill and occupies a slot for the whole run.
	fast := []string{"fast-1", "fast-2", "fast-3", "fast-4", "fast-5", "fast-6", "fast-7", "fast-8"}
	src := newPoolBatchSource(append([]string{"aaa-straggler"}, fast...)...)
	env.OnActivity(a.NextUnitBatch, mock.Anything, mock.Anything).Return(src.batch)

	var (
		mu         sync.Mutex
		fastSynced = map[string]bool{}
	)
	env.OnActivity(a.SyncUnit, mock.Anything, mock.Anything).
		Return(func(ctx context.Context, in activity.SyncInput) (activity.SyncResult, error) {
			if in.UnitID == "aaa-straggler" {
				// The slow repo: it does not return until the drain is cancelled (the
				// cooperative stop). Under the barrier this wedges the whole slice; under
				// the pool it just parks one slot while the others keep flowing.
				<-ctx.Done()
				return activity.SyncResult{Done: false, Status: "syncing"}, nil
			}
			mu.Lock()
			fastSynced[in.UnitID] = true
			done := len(fastSynced) == len(fast)
			mu.Unlock()
			if done {
				// All fast units have drained past the straggler — stop the run so the
				// blocked straggler is released and the workflow can settle.
				env.SignalWorkflow(stopSync, nil)
			}
			src.retire(in.UnitID)
			return activity.SyncResult{Done: true, Status: "synced"}, nil
		})

	env.RegisterDelayedCallback(func() { env.SignalWorkflow(triggerSync, nil) }, 0)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(disconnect, nil) }, time.Second)

	env.ExecuteWorkflow(ConnectionWorkflow, testInput())

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	// The load-bearing assertion: ALL 8 fast units retired while the straggler was still
	// in flight. That is impossible under the batch-barrier (at most P-1 = 3 fast units
	// run in the wedged slice) — it proves the pool refilled freed slots past the straggler.
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, fastSynced, len(fast), "every fast unit drained past the in-flight straggler (the pool refilled freed slots)")
}

// TestStopSyncCancelsAndSettles: a StopSync mid-drain cancels the drain and settles
// without a fresh run. SyncUnit never completes a unit on its own (Done=false, unit
// stays pending), so the drain only ends via the cooperative stop; the workflow must
// still call SettleConnection and then end (no continueAsNew rerun).
func TestStopSyncCancelsAndSettles(t *testing.T) {
	s := &testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	a := &activity.Activities{}

	// Control-plane stubs, with a settle override that records it ran (the only
	// per-test stub for SettleConnection — registering it twice would shadow this one).
	env.OnActivity(a.StartRun, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(a.DiscoverUnits, mock.Anything, mock.Anything).
		Return(activity.DiscoverResult{Count: 0}, nil)
	settled := false
	env.OnActivity(a.SettleConnection, mock.Anything, mock.Anything).
		Return(func(_ context.Context, _ activity.SettleInput) (activity.SettleResult, error) {
			settled = true
			return activity.SettleResult{Status: "connected"}, nil
		})
	mockSchedule(env, true, defaultSyncIntervalSeconds)
	mockCapacity(env, 4)

	// The table always has work and SyncUnit never finishes a unit (Done=false): the
	// unit retires from the pool's in-flight set but stays pending, so the pool keeps
	// re-launching it. The drain therefore only ends via the cooperative stop — exactly
	// the case under test. drainSignals() picks the stop up at the next completion
	// boundary, cancels the drain context, and the pool drains and settles (ADR-0040).
	env.OnActivity(a.NextUnitBatch, mock.Anything, mock.Anything).
		Return(activity.BatchResult{UnitIDs: []string{"r1"}}, nil)
	env.OnActivity(a.SyncUnit, mock.Anything, mock.Anything).
		Return(activity.SyncResult{Done: false, Status: "syncing"}, nil)

	// Trigger the run, then stop it (cooperative cancel at the next completion boundary),
	// then disconnect to end the workflow. Same logical time → fired in this order; the
	// signals are buffered and drainSignals() picks the stop up at a completion boundary.
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(triggerSync, nil) }, 0)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(stopSync, nil) }, 0)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(disconnect, nil) }, time.Second)

	env.ExecuteWorkflow(ConnectionWorkflow, testInput())

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.True(t, settled, "a stopped run still settles to rest")
	env.AssertExpectations(t)
}

// TestContinueAsNewAtProcessedBound: the run continueAsNews once it has processed the
// unit bound. We return a full set of fresh units every refill so processed climbs past
// continueAsNewUnits without the drain ever emptying; the workflow must return a
// ContinueAsNew error (the test env surfaces it as the workflow result, and does NOT
// recurse). With the pool, in-flight units are drained before the continueAsNew.
func TestContinueAsNewAtProcessedBound(t *testing.T) {
	s := &testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	a := &activity.Activities{}
	mockControlActivities(env)

	// Never-emptying table: NextUnitBatch always returns P fresh, distinct ids (keyed by
	// a counter) so the pool keeps launching and processed climbs past the bound (1000).
	counter := 0
	env.OnActivity(a.NextUnitBatch, mock.Anything, mock.Anything).
		Return(func(_ context.Context, in activity.BatchInput) (activity.BatchResult, error) {
			ids := make([]string, 0, in.Limit)
			for i := 0; i < in.Limit; i++ {
				counter++
				ids = append(ids, "u"+itoa(counter))
			}
			return activity.BatchResult{UnitIDs: ids}, nil
		})
	env.OnActivity(a.SyncUnit, mock.Anything, mock.Anything).
		Return(activity.SyncResult{Done: true, Status: "synced"}, nil)

	env.RegisterDelayedCallback(func() { env.SignalWorkflow(triggerSync, nil) }, 0)

	env.ExecuteWorkflow(ConnectionWorkflow, testInput())

	require.True(t, env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	require.Error(t, err, "the workflow continueAsNews at the processed bound")
	var canErr *workflow.ContinueAsNewError
	require.True(t, errors.As(err, &canErr), "error is a ContinueAsNewError: %v", err)
}

// TestAutoSyncDisabledArmsNoTimer: with auto-sync off, the idle wait arms NO periodic
// timer — the workflow runs only when a signal lands. We disable auto-sync, advance the
// clock far past the default cadence with no trigger, and assert StartRun never ran (no
// run was driven). A Disconnect then ends the workflow. If a timer had been armed, the
// elapsed clock would have driven a run and StartRun would have fired.
func TestAutoSyncDisabledArmsNoTimer(t *testing.T) {
	s := &testsuite.WorkflowTestSuite{}
	env := s.NewTestWorkflowEnvironment()
	a := &activity.Activities{}

	mockSchedule(env, false, defaultSyncIntervalSeconds)
	started := false
	env.OnActivity(a.StartRun, mock.Anything, mock.Anything).
		Return(func(_ context.Context, _ activity.RunInput) error {
			started = true
			return nil
		}).Maybe()

	// No trigger — just let the clock run past the default cadence, then disconnect.
	env.RegisterDelayedCallback(func() { env.SignalWorkflow(disconnect, nil) }, time.Hour)

	env.ExecuteWorkflow(ConnectionWorkflow, testInput())

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.False(t, started, "auto-sync off arms no timer, so no run was driven")
}

// itoa is a tiny non-allocating-ish int→string for fresh unit ids in the bound test
// (strconv would do; this keeps the test's intent — distinct ids — obvious).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
