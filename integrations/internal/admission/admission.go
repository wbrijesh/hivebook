// Package admission is the one multi-dimensional adaptive admission controller
// (ADR-0036). It is a single AIMD limiter over the admission_bucket table, used at
// two scopes from the same primitive: 'credential' (scope_key = connection_id, the
// adaptive per-token API budget L) and 'tenant' (scope_key = tenant_id, a hard
// in-flight cap for fairness). The connector seeds the starting concurrency and hard
// caps via RateBudget(); the real ceiling is learned at runtime — additive increase on
// sustained success, multiplicative decrease + cooldown on a throttle.
package admission

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"integrations/internal/connectors"
	"integrations/internal/database/gen"
)

// Scope types — the two dimensions of the one limiter (ADR-0036).
const (
	ScopeCredential = "credential" // scope_key = connection_id: the adaptive API budget
	ScopeTenant     = "tenant"     // scope_key = tenant_id: the hard in-flight cap
)

// Limiter is the AIMD controller over admission_bucket. It is stateless beyond the
// queries it runs — all state (L, in_flight, cooldown) lives in Postgres, so the
// limiter is shared safely across worker processes and the acquire is a DB-atomic CAS.
type Limiter struct {
	q *gen.Queries
}

// New builds a Limiter over the generated query set.
func New(q *gen.Queries) *Limiter { return &Limiter{q: q} }

// Acquire lazily seeds the bucket on first use (concurrency_limit = seed.StartConcurrency),
// then atomically takes a slot iff one is free and no cooldown is in effect. The acquire
// is a single UPDATE … WHERE in_flight < concurrency_limit … RETURNING (a compare-and-swap):
// no row updated means the limit is hit or the bucket is in cooldown, and ok is false.
// The caller (SyncUnit) turns !ok into a retryable Temporal error so the unit re-schedules.
func (l *Limiter) Acquire(ctx context.Context, scopeType, scopeKey, connectorID string, seed connectors.RateBudget) (bool, error) {
	start := seed.StartConcurrency
	if start < 1 {
		start = 1
	}
	if err := l.q.SeedAdmissionBucket(ctx, gen.SeedAdmissionBucketParams{
		ScopeType:        scopeType,
		ScopeKey:         scopeKey,
		ConnectorID:      connectorID,
		ConcurrencyLimit: int32(start),
	}); err != nil {
		return false, err
	}
	_, err := l.q.AcquireAdmission(ctx, gen.AcquireAdmissionParams{ScopeType: scopeType, ScopeKey: scopeKey})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // limit hit or cooling down — no slot
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Release frees a slot (clamped at zero). Always paired with a successful Acquire,
// typically via defer.
func (l *Limiter) Release(ctx context.Context, scopeType, scopeKey string) error {
	return l.q.ReleaseAdmission(ctx, gen.ReleaseAdmissionParams{ScopeType: scopeType, ScopeKey: scopeKey})
}

// RecordSuccess is the additive increase: on a clean drain, grow L by one toward the
// connector's hard cap (max), but only while in_flight is near the limit — an idle
// bucket must not drift its limit upward without real load behind it.
func (l *Limiter) RecordSuccess(ctx context.Context, scopeType, scopeKey string, max int) error {
	if max < 1 {
		max = 1
	}
	return l.q.AdmissionAdditiveIncrease(ctx, gen.AdmissionAdditiveIncreaseParams{
		MaxLimit:  int32(max),
		ScopeType: scopeType,
		ScopeKey:  scopeKey,
	})
}

// RecordThrottle is the multiplicative decrease: on a 429 / secondary-403 / Retry-After,
// halve L (floor 1) and park the bucket in cooldown for retryAfter so no new unit acquires
// until the source recovers.
func (l *Limiter) RecordThrottle(ctx context.Context, scopeType, scopeKey string, retryAfter time.Duration) error {
	var cooldown sql.NullTime
	if retryAfter > 0 {
		cooldown = sql.NullTime{Time: time.Now().Add(retryAfter), Valid: true}
	}
	return l.q.AdmissionMultiplicativeDecrease(ctx, gen.AdmissionMultiplicativeDecreaseParams{
		CooldownUntil: cooldown,
		ScopeType:     scopeType,
		ScopeKey:      scopeKey,
	})
}

// CurrentLimit reports the bucket's learned limit L for the workflow's Capacity macro.
// A bucket that has never been seeded returns (0, nil) — the caller seeds + floors it.
func (l *Limiter) CurrentLimit(ctx context.Context, scopeType, scopeKey string) (int, error) {
	v, err := l.q.GetAdmissionLimit(ctx, gen.GetAdmissionLimitParams{ScopeType: scopeType, ScopeKey: scopeKey})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// SeedTenant lazily creates a tenant-scope bucket at a default in-flight cap. The tenant
// limiter is the same primitive as the credential one, just keyed by tenant_id; it is
// seeded from a platform constant (the connector seed is per-credential, not per-tenant).
func (l *Limiter) SeedTenant(ctx context.Context, tenantID string, defaultCap int) error {
	if defaultCap < 1 {
		defaultCap = 1
	}
	return l.q.SeedAdmissionBucket(ctx, gen.SeedAdmissionBucketParams{
		ScopeType:        ScopeTenant,
		ScopeKey:         tenantID,
		ConnectorID:      "",
		ConcurrencyLimit: int32(defaultCap),
	})
}
