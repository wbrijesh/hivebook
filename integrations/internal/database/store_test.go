package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"integrations/internal/database/gen"
)

// newStore opens a store against the TestMain container (migrations run on New)
// and closes it when the test ends. Tests isolate by a unique tenant per test.
func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(testCfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func newConnection(t *testing.T, s *Store, tenant string) gen.ConnectorConnection {
	t.Helper()
	row, err := s.Q.UpsertConnection(context.Background(), gen.UpsertConnectionParams{
		TenantID:     tenant,
		Region:       "us-east",
		ConnectorID:  "gdocs",
		Account:      "ops@acme.com",
		AccessToken:  []byte("enc-access"),
		RefreshToken: []byte("enc-refresh"),
		TokenExpiry:  sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true},
		Scopes:       "drive.readonly",
	})
	if err != nil {
		t.Fatalf("UpsertConnection: %v", err)
	}
	return row
}

func TestConnectionLifecycle(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tenant := "org_" + t.Name()

	row := newConnection(t, s, tenant)
	if row.ID == "" || row.Status != "connected" {
		t.Fatalf("unexpected new connection: %+v", row)
	}

	got, err := s.Q.GetConnectionByID(ctx, row.ID)
	if err != nil || got.Account != "ops@acme.com" {
		t.Fatalf("GetConnectionByID: %v %+v", err, got)
	}

	if n, err := s.Q.DeleteConnection(ctx, gen.DeleteConnectionParams{ID: row.ID, TenantID: tenant}); err != nil || n != 1 {
		t.Fatalf("DeleteConnection: n=%d err=%v", n, err)
	}
	list, _ := s.Q.ListConnections(ctx, tenant)
	if len(list) != 0 {
		t.Fatalf("connection should be gone, got %d", len(list))
	}
}

func TestSetNeedsReauth(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	row := newConnection(t, s, "org_"+t.Name())

	if err := s.Q.SetNeedsReauth(ctx, gen.SetNeedsReauthParams{ID: row.ID, LastError: "invalid_grant"}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Q.GetConnectionByID(ctx, row.ID)
	if got.Status != "needs_reauth" || got.LastError != "invalid_grant" {
		t.Fatalf("needs_reauth not recorded: status=%q err=%q", got.Status, got.LastError)
	}
}

func TestRefreshTokenNotClobbered(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tenant := "org_" + t.Name()

	newConnection(t, s, tenant) // has refresh-token "enc-refresh"
	// Reconnect with an empty refresh token (Google omits it without consent).
	row, err := s.Q.UpsertConnection(ctx, gen.UpsertConnectionParams{
		TenantID: tenant, Region: "us-east", ConnectorID: "gdocs", Account: "ops@acme.com",
		AccessToken: []byte("new-access"), RefreshToken: nil,
		TokenExpiry: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}, Scopes: "drive.readonly",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(row.RefreshToken) != "enc-refresh" {
		t.Fatalf("refresh token must be preserved, got %q", row.RefreshToken)
	}
}

// TestSyncUnitCursorRoundTrip locks the high-water-mark cursor (ADR-0031/0038): the
// cursor lives on the unit, per unit — no global cursor.
func TestSyncUnitCursorRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	row := newConnection(t, s, "org_"+t.Name())

	if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{
		ConnectionID: row.ID, ExternalID: "acme/api", Name: "api", Kind: "repo", Generation: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.SetUnitCursor(ctx, gen.SetUnitCursorParams{ConnectionID: row.ID, ExternalID: "acme/api", Cursor: "cursor1"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: row.ID, ExternalID: "acme/api"})
	if err != nil || got.Cursor != "cursor1" {
		t.Fatalf("GetSyncUnit: %v cursor=%q", err, got.Cursor)
	}
	// A different unit has its own cursor (no global cursor — review #1).
	if _, err := s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: row.ID, ExternalID: "acme/web"}); err != sql.ErrNoRows {
		t.Fatalf("expected no unit for a different external_id, got %v", err)
	}
}

func TestRawArtifactDedup(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	row := newConnection(t, s, "org_"+t.Name())

	upsert := func(hash string) {
		if err := s.Q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
			TenantID: row.TenantID, Region: row.Region, ConnectionID: row.ID,
			Source: "gdocs", SourceNativeKind: "gdoc", ExternalID: "doc1",
			ObjectKey: "bucket/tenant/doc1/" + hash, ContentHash: hash,
		}); err != nil {
			t.Fatalf("UpsertRawArtifact: %v", err)
		}
	}
	upsert("hash-v1")
	got, err := s.Q.GetArtifactContentHash(ctx, gen.GetArtifactContentHashParams{ConnectionID: row.ID, ExternalID: "doc1"})
	if err != nil || got != "hash-v1" {
		t.Fatalf("hash v1: %v %q", err, got)
	}
	upsert("hash-v2")
	got, _ = s.Q.GetArtifactContentHash(ctx, gen.GetArtifactContentHashParams{ConnectionID: row.ID, ExternalID: "doc1"})
	if got != "hash-v2" {
		t.Fatalf("hash should update to v2, got %q", got)
	}
}

// TestConnectionArtifactDeletionIsTenantScoped locks the P0 tenant-isolation fix:
// a connection id is not a secret, so one tenant must never delete another's
// connection or soft-delete its artifacts (design-doc 0011).
func TestConnectionArtifactDeletionIsTenantScoped(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tenantA := "org_A_" + t.Name()
	tenantB := "org_B_" + t.Name()
	conn := newConnection(t, s, tenantA)

	// One visible artifact owned by tenant A (committed unit, matching sync id).
	if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{ConnectionID: conn.ID, ExternalID: "c1", Name: "c1", Kind: "k", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
		TenantID: tenantA, Region: "us-east", ConnectionID: conn.ID, Source: "gdocs",
		SourceNativeKind: "gdoc", ContainerID: "c1", SyncContainerID: "c1", ExternalID: "a1",
		ObjectKey: "b/a1", ContentHash: "h1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.CommitUnit(ctx, gen.CommitUnitParams{ConnectionID: conn.ID, ExternalID: "c1"}); err != nil {
		t.Fatal(err)
	}
	visibleToA := func() int {
		n, err := s.Q.CountArtifacts(ctx, gen.CountArtifactsParams{TenantID: tenantA})
		if err != nil {
			t.Fatal(err)
		}
		return int(n)
	}
	if visibleToA() != 1 {
		t.Fatalf("tenant A should see its artifact, got %d", visibleToA())
	}

	// Tenant B cannot delete tenant A's connection (0 rows, no error)...
	if n, err := s.Q.DeleteConnection(ctx, gen.DeleteConnectionParams{ID: conn.ID, TenantID: tenantB}); err != nil || n != 0 {
		t.Fatalf("cross-tenant DeleteConnection must affect 0 rows: n=%d err=%v", n, err)
	}
	// ...nor soft-delete its artifacts.
	if err := s.Q.SoftDeleteConnectionArtifacts(ctx, gen.SoftDeleteConnectionArtifactsParams{ConnectionID: conn.ID, TenantID: tenantB}); err != nil {
		t.Fatal(err)
	}
	if visibleToA() != 1 {
		t.Fatalf("tenant B must NOT soft-delete tenant A's artifacts, got %d", visibleToA())
	}

	// The owner can.
	if err := s.Q.SoftDeleteConnectionArtifacts(ctx, gen.SoftDeleteConnectionArtifactsParams{ConnectionID: conn.ID, TenantID: tenantA}); err != nil {
		t.Fatal(err)
	}
	if visibleToA() != 0 {
		t.Fatalf("owner soft-delete should hide the artifact, got %d", visibleToA())
	}
}

// TestArtifactVisibilityJoinsOnSyncUnit locks the P0 visibility fix: the
// committed_at gate joins on the SYNC unit (the unit's external_id), not the
// manifest container. For gdocs these differ (drive vs folder); joining on the
// manifest container would hide every gdoc (ADR-0033).
func TestArtifactVisibilityJoinsOnSyncUnit(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tenant := "org_" + t.Name()
	conn := newConnection(t, s, tenant)

	// gdocs shape: unit = "drive"; artifact's manifest container = a folder.
	if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{ConnectionID: conn.ID, ExternalID: "drive", Name: "Drive", Kind: "google_drive", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
		TenantID: tenant, Region: "us-east", ConnectionID: conn.ID, Source: "gdocs",
		SourceNativeKind: "gdoc", ContainerID: "folder-xyz", SyncContainerID: "drive", ExternalID: "doc1",
		ObjectKey: "b/doc1", ContentHash: "h1",
	}); err != nil {
		t.Fatal(err)
	}
	count := func() int {
		n, err := s.Q.CountArtifacts(ctx, gen.CountArtifactsParams{TenantID: tenant})
		if err != nil {
			t.Fatal(err)
		}
		return int(n)
	}

	// Uncommitted unit → hidden (no partial data exposed).
	if count() != 0 {
		t.Fatalf("uncommitted unit's artifacts must be hidden, got %d", count())
	}
	// First full pass completes → committed_at set → visible, even though the
	// artifact's container_id ("folder-xyz") != the unit's external_id ("drive").
	if err := s.Q.CommitUnit(ctx, gen.CommitUnitParams{ConnectionID: conn.ID, ExternalID: "drive"}); err != nil {
		t.Fatal(err)
	}
	if count() != 1 {
		t.Fatalf("committed gdocs artifact must be visible despite folder!=drive, got %d", count())
	}
	rows, err := s.Q.ListArtifacts(ctx, gen.ListArtifactsParams{TenantID: tenant, RowLimit: 50, RowOffset: 0})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListArtifacts: len=%d err=%v", len(rows), err)
	}
	// committed_at is idempotent — re-committing keeps the first commit and the count.
	if err := s.Q.CommitUnit(ctx, gen.CommitUnitParams{ConnectionID: conn.ID, ExternalID: "drive"}); err != nil {
		t.Fatal(err)
	}
	if u, _ := s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: conn.ID, ExternalID: "drive"}); u.ArtifactCount != 1 {
		t.Fatalf("artifact_count should recompute to 1, got %d", u.ArtifactCount)
	}
}

// TestUpdateAccessTokenPreservesRefresh locks P1-1: a rotation that returns no new
// refresh token must persist the access token without blanking the stored refresh.
func TestUpdateAccessTokenPreservesRefresh(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	row := newConnection(t, s, "org_"+t.Name()) // seeded with refresh "enc-refresh"

	if err := s.Q.UpdateAccessToken(ctx, gen.UpdateAccessTokenParams{
		ID:          row.ID,
		AccessToken: []byte("new-access"),
		TokenExpiry: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("UpdateAccessToken: %v", err)
	}
	got, err := s.Q.GetConnectionByID(ctx, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.AccessToken) != "new-access" {
		t.Fatalf("access token should update, got %q", got.AccessToken)
	}
	if string(got.RefreshToken) != "enc-refresh" {
		t.Fatalf("refresh token must be preserved, got %q", got.RefreshToken)
	}
}

// TestConsecutiveFailuresCountsRuns locks the run-level streak (design-doc 0012): a
// failed run bumps consecutive_failures once; a clean run resets it and settles
// connected. The Temporal SETTLE step calls RecordSyncFailure/Success once per run.
func TestConsecutiveFailuresCountsRuns(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	row := newConnection(t, s, "org_"+t.Name())

	failedRun := func() {
		if err := s.Q.RecordSyncAttempt(ctx, row.ID); err != nil {
			t.Fatal(err)
		}
		if err := s.Q.RecordSyncFailure(ctx, gen.RecordSyncFailureParams{ID: row.ID, LastError: "boom"}); err != nil {
			t.Fatal(err)
		}
	}

	failedRun()
	if got, _ := s.Q.GetConnectionByID(ctx, row.ID); got.ConsecutiveFailures != 1 || got.Status != "error" {
		t.Fatalf("after 1 failed run: failures=%d status=%q", got.ConsecutiveFailures, got.Status)
	}
	failedRun()
	if got, _ := s.Q.GetConnectionByID(ctx, row.ID); got.ConsecutiveFailures != 2 {
		t.Fatalf("after 2 failed runs: failures=%d (want 2)", got.ConsecutiveFailures)
	}

	// A clean run resets the streak and settles connected.
	if err := s.Q.RecordSyncAttempt(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.RecordSyncSuccess(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Q.GetConnectionByID(ctx, row.ID); got.ConsecutiveFailures != 0 || got.Status != "connected" || !got.LastSuccessAt.Valid {
		t.Fatalf("after clean run: failures=%d status=%q success=%v", got.ConsecutiveFailures, got.Status, got.LastSuccessAt.Valid)
	}
}

// TestUnitSelectionGatesArtifacts locks the selection → visibility flow: deselecting
// a unit soft-deletes its files; re-selecting un-deletes them (design-doc 0011).
func TestUnitSelectionGatesArtifacts(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tenant := "org_" + t.Name()
	conn := newConnection(t, s, tenant)

	for _, id := range []string{"u1", "u2"} {
		if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{ConnectionID: conn.ID, ExternalID: id, Name: id, Kind: "repo", Generation: 1}); err != nil {
			t.Fatal(err)
		}
		if err := s.Q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
			TenantID: tenant, Region: "us-east", ConnectionID: conn.ID, Source: "github",
			SourceNativeKind: "issue", ContainerID: id, SyncContainerID: id, ExternalID: "art-" + id,
			ObjectKey: "b/" + id, ContentHash: "h",
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.Q.CommitUnit(ctx, gen.CommitUnitParams{ConnectionID: conn.ID, ExternalID: id}); err != nil {
			t.Fatal(err)
		}
	}
	count := func() int {
		n, _ := s.Q.CountArtifacts(ctx, gen.CountArtifactsParams{TenantID: tenant})
		return int(n)
	}
	if count() != 2 {
		t.Fatalf("both committed units visible, got %d", count())
	}

	// Select only u1; u2's artifact is soft-deleted (hidden).
	if err := s.Q.SetUnitSelection(ctx, gen.SetUnitSelectionParams{ConnectionID: conn.ID, SelectedIds: []string{"u1"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.SoftDeleteDeselectedUnitArtifacts(ctx, conn.ID); err != nil {
		t.Fatal(err)
	}
	if count() != 1 {
		t.Fatalf("deselected unit's artifact must be hidden, got %d", count())
	}

	// Re-select u2; its artifact comes back.
	if err := s.Q.SetUnitSelection(ctx, gen.SetUnitSelectionParams{ConnectionID: conn.ID, SelectedIds: []string{"u1", "u2"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.UndeleteSelectedUnitArtifacts(ctx, conn.ID); err != nil {
		t.Fatal(err)
	}
	if count() != 2 {
		t.Fatalf("re-selected unit's artifact must reappear, got %d", count())
	}
}

// TestMarkAndSweepDiscovery locks ADR-0037: a later-generation discovery marks the
// units it sees; the sweep tombstones the unseen ones (status='removed'); GC drops
// tombstones past the grace cutoff.
func TestMarkAndSweepDiscovery(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	conn := newConnection(t, s, "org_"+t.Name())

	// Generation 1 discovers two units.
	for _, id := range []string{"repo-a", "repo-b"} {
		if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{ConnectionID: conn.ID, ExternalID: id, Name: id, Kind: "repo", Generation: 1}); err != nil {
			t.Fatal(err)
		}
	}
	// Generation 2 re-discovers only repo-a (repo-b is gone from the source).
	if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{ConnectionID: conn.ID, ExternalID: "repo-a", Name: "repo-a", Kind: "repo", Generation: 2}); err != nil {
		t.Fatal(err)
	}
	n, err := s.Q.SweepUnseenUnits(ctx, gen.SweepUnseenUnitsParams{ConnectionID: conn.ID, Generation: 2})
	if err != nil || n != 1 {
		t.Fatalf("sweep should tombstone exactly repo-b: n=%d err=%v", n, err)
	}
	if u, _ := s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: conn.ID, ExternalID: "repo-b"}); u.Status != "removed" {
		t.Fatalf("repo-b should be tombstoned, status=%q", u.Status)
	}
	if u, _ := s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: conn.ID, ExternalID: "repo-a"}); u.Status == "removed" {
		t.Fatal("repo-a was seen this generation and must not be tombstoned")
	}

	// GC drops tombstones older than the cutoff (use a future cutoff to catch it now).
	gc, err := s.Q.GCRemovedUnits(ctx, gen.GCRemovedUnitsParams{ConnectionID: conn.ID, Cutoff: time.Now().Add(time.Hour)})
	if err != nil || gc != 1 {
		t.Fatalf("GC should drop repo-b: gc=%d err=%v", gc, err)
	}
	if _, err := s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: conn.ID, ExternalID: "repo-b"}); err != sql.ErrNoRows {
		t.Fatalf("repo-b should be gone after GC, got %v", err)
	}
}

// TestPoisonUnitBackoff locks the per-unit retry backoff (ADR-0036): a failing unit
// arms next_retry_at and records the error; a later success clears both.
func TestPoisonUnitBackoff(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	conn := newConnection(t, s, "org_"+t.Name())

	if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{ConnectionID: conn.ID, ExternalID: "bad-repo", Name: "bad", Kind: "repo", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	retryAt := sql.NullTime{Time: time.Now().Add(5 * time.Minute), Valid: true}
	if err := s.Q.SetUnitRetry(ctx, gen.SetUnitRetryParams{ConnectionID: conn.ID, ExternalID: "bad-repo", LastError: "rate limited", NextRetryAt: retryAt}); err != nil {
		t.Fatal(err)
	}
	u, _ := s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: conn.ID, ExternalID: "bad-repo"})
	if u.Status != "error" || u.LastError != "rate limited" || !u.NextRetryAt.Valid {
		t.Fatalf("backoff not armed: status=%q err=%q retry=%v", u.Status, u.LastError, u.NextRetryAt.Valid)
	}
	if err := s.Q.ClearUnitRetry(ctx, gen.ClearUnitRetryParams{ConnectionID: conn.ID, ExternalID: "bad-repo"}); err != nil {
		t.Fatal(err)
	}
	u, _ = s.Q.GetSyncUnit(ctx, gen.GetSyncUnitParams{ConnectionID: conn.ID, ExternalID: "bad-repo"})
	if u.LastError != "" || u.NextRetryAt.Valid {
		t.Fatalf("backoff not cleared: err=%q retry=%v", u.LastError, u.NextRetryAt.Valid)
	}
}

// TestErasureDeletionOutbox locks ADR-0039: a delete marks the row purge_pending
// (not dropped) with an SLA deadline; the outbox lists it; confirm drops it; an
// overdue row surfaces for paging.
func TestErasureDeletionOutbox(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	tenant := "org_" + t.Name()
	conn := newConnection(t, s, tenant)

	if err := s.Q.UpsertSyncUnit(ctx, gen.UpsertSyncUnitParams{ConnectionID: conn.ID, ExternalID: "u1", Name: "u1", Kind: "repo", Generation: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
		TenantID: tenant, Region: "us-east", ConnectionID: conn.ID, Source: "github",
		SourceNativeKind: "issue", ContainerID: "u1", SyncContainerID: "u1", ExternalID: "art1",
		ObjectKey: "b/art1", ContentHash: "h",
	}); err != nil {
		t.Fatal(err)
	}
	// Resolve the artifact id via the manifest list (committed first so it lists).
	if err := s.Q.CommitUnit(ctx, gen.CommitUnitParams{ConnectionID: conn.ID, ExternalID: "u1"}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Q.ListArtifacts(ctx, gen.ListArtifactsParams{TenantID: tenant, RowLimit: 10})
	if err != nil || len(rows) != 1 {
		t.Fatalf("expected 1 artifact, len=%d err=%v", len(rows), err)
	}
	artID := rows[0].ID

	// Mark for purge with a past deadline → it appears in the outbox AND in the
	// past-deadline (page) list, and is hidden from Files.
	past := sql.NullTime{Time: time.Now().Add(-time.Hour), Valid: true}
	if err := s.Q.MarkArtifactsPurgePending(ctx, gen.MarkArtifactsPurgePendingParams{Ids: []string{artID}, PurgeDeadline: past}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.Q.CountArtifacts(ctx, gen.CountArtifactsParams{TenantID: tenant}); n != 0 {
		t.Fatalf("purge-pending artifact must be hidden, got %d", n)
	}
	pend, err := s.Q.ListPurgePendingArtifacts(ctx, 10)
	if err != nil || len(pend) != 1 || pend[0].ID != artID {
		t.Fatalf("outbox should list the pending artifact: %+v err=%v", pend, err)
	}
	// The SLA list keys on deleted_at (the purge-REQUEST time), not purge_deadline. The
	// row was just marked (deleted_at = now), so an sla_cutoff in the future surfaces it
	// as breached; a past cutoff would not — exercise the breach path with a future cutoff.
	overdue, err := s.Q.ListPurgePendingPastDeadline(ctx, gen.ListPurgePendingPastDeadlineParams{
		SlaCutoff: sql.NullTime{Time: time.Now().Add(time.Hour), Valid: true}, RowLimit: 10,
	})
	if err != nil || len(overdue) != 1 {
		t.Fatalf("overdue list should surface the SLA breach: len=%d err=%v", len(overdue), err)
	}

	// Confirm the object delete → the manifest row is dropped.
	got, err := s.Q.ConfirmArtifactDeletes(ctx, []string{artID})
	if err != nil || got != 1 {
		t.Fatalf("ConfirmArtifactDeletes: n=%d err=%v", got, err)
	}
	if pend, _ := s.Q.ListPurgePendingArtifacts(ctx, 10); len(pend) != 0 {
		t.Fatalf("outbox should be empty after confirm, got %d", len(pend))
	}
}

// TestAdmissionBucketCRUD locks the basic ADR-0036 bucket CRUD (the AIMD logic is
// Phase 3): seed, read, adjust the limit, set in-flight, delete.
func TestAdmissionBucketCRUD(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	conn := newConnection(t, s, "org_"+t.Name())

	if err := s.Q.UpsertAdmissionBucket(ctx, gen.UpsertAdmissionBucketParams{
		ScopeType: "credential", ScopeKey: conn.ID, ConnectorID: "github", ConcurrencyLimit: 4,
	}); err != nil {
		t.Fatal(err)
	}
	b, err := s.Q.GetAdmissionBucket(ctx, gen.GetAdmissionBucketParams{ScopeType: "credential", ScopeKey: conn.ID})
	if err != nil || b.ConcurrencyLimit != 4 || b.InFlight != 0 {
		t.Fatalf("GetAdmissionBucket: %+v err=%v", b, err)
	}
	// AIMD adjusts the limit with a cooldown; in-flight accounting moves separately.
	cool := sql.NullTime{Time: time.Now().Add(time.Minute), Valid: true}
	if err := s.Q.SetAdmissionLimit(ctx, gen.SetAdmissionLimitParams{ScopeType: "credential", ScopeKey: conn.ID, ConcurrencyLimit: 2, CooldownUntil: cool}); err != nil {
		t.Fatal(err)
	}
	if err := s.Q.SetAdmissionInFlight(ctx, gen.SetAdmissionInFlightParams{ScopeType: "credential", ScopeKey: conn.ID, InFlight: 1}); err != nil {
		t.Fatal(err)
	}
	b, _ = s.Q.GetAdmissionBucket(ctx, gen.GetAdmissionBucketParams{ScopeType: "credential", ScopeKey: conn.ID})
	if b.ConcurrencyLimit != 2 || b.InFlight != 1 || !b.CooldownUntil.Valid {
		t.Fatalf("bucket after adjust: %+v", b)
	}
	if err := s.Q.DeleteAdmissionBucket(ctx, gen.DeleteAdmissionBucketParams{ScopeType: "credential", ScopeKey: conn.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Q.GetAdmissionBucket(ctx, gen.GetAdmissionBucketParams{ScopeType: "credential", ScopeKey: conn.ID}); err != sql.ErrNoRows {
		t.Fatalf("bucket should be gone, got %v", err)
	}
}
