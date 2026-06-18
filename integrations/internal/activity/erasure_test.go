package activity

import (
	"context"
	"testing"
	"time"

	"integrations/internal/database/gen"
)

// TestSettleStatusRollup locks the phase × unit-status fold (design-doc 0012). The pure
// function is tested directly so the rollup rules are pinned without a DB.
func TestSettleStatusRollup(t *testing.T) {
	cases := []struct {
		name                        string
		total, errored, uncommitted int
		want                        string
	}{
		{"nothing selected", 0, 0, 0, "connected"},
		{"all committed, none errored", 5, 0, 0, "connected"},
		{"all committed, some errored", 5, 2, 0, "connected_warnings"},
		{"initial backfill in flight", 5, 0, 3, "backfilling"},
		{"backfill with one errored unit still has uncommitted progress", 5, 1, 3, "backfilling"},
		{"every selected unit errored", 4, 4, 4, "error"},
		{"only the uncommitted ones are errored → not backfilling", 5, 2, 2, "connected_warnings"},
	}
	for _, c := range cases {
		if got := settleStatus(c.total, c.errored, c.uncommitted); got != c.want {
			t.Errorf("%s: settleStatus(%d,%d,%d)=%q want %q",
				c.name, c.total, c.errored, c.uncommitted, got, c.want)
		}
	}
}

// TestPurgeArtifactsConfirmsDeletes locks the erasure outbox happy path (ADR-0039): a
// purge_pending artifact whose deadline has elapsed is deleted from storage and its
// manifest row dropped only after the delete confirms.
func TestPurgeArtifactsConfirmsDeletes(t *testing.T) {
	acts, store, _, connID := setup(t)
	ctx := context.Background()
	seedUnit(t, store, connID)

	// One committed artifact, then marked purge_pending with a past deadline.
	if err := store.Q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
		TenantID: "tenant-x", Region: "us", ConnectionID: connID, Source: "github",
		SourceNativeKind: "issue", ContainerID: "acme/api", SyncContainerID: "acme/api",
		ExternalID: "art-1", ObjectKey: "bucket/obj-1", ContentHash: "h",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := store.Q.ListArtifacts(ctx, gen.ListArtifactsParams{TenantID: "tenant-x", RowLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Commit the unit so the artifact is resolvable; then mark it for purge.
	if err := store.Q.CommitUnit(ctx, gen.CommitUnitParams{ConnectionID: connID, ExternalID: "acme/api"}); err != nil {
		t.Fatal(err)
	}
	rows, err = store.Q.ListArtifacts(ctx, gen.ListArtifactsParams{TenantID: "tenant-x", RowLimit: 10})
	if err != nil || len(rows) != 1 {
		t.Fatalf("expected 1 artifact, len=%d err=%v", len(rows), err)
	}
	artID := rows[0].ID
	if err := store.Q.MarkArtifactsPurgePending(ctx, gen.MarkArtifactsPurgePendingParams{
		Ids: []string{artID}, PurgeDeadline: nullTimeVal(time.Now().Add(-time.Hour)),
	}); err != nil {
		t.Fatal(err)
	}

	res, err := acts.PurgeArtifacts(ctx, PurgeInput{Limit: 100})
	if err != nil {
		t.Fatalf("PurgeArtifacts: %v", err)
	}
	if res.Deleted != 1 || res.Failed != 0 {
		t.Fatalf("expected 1 deleted, 0 failed: %+v", res)
	}
	// The row is gone (delete confirmed) — the outbox is empty.
	if pend, _ := store.Q.ListPurgePendingArtifacts(ctx, 10); len(pend) != 0 {
		t.Fatalf("outbox must be empty after a confirmed purge, got %d", len(pend))
	}
}

// TestMarkConnectionPurgeImmediate locks the disconnect erasure path (ADR-0039): all of
// a connection's artifacts are marked purge_pending immediately (no grace), tenant-scoped.
func TestMarkConnectionPurgeImmediate(t *testing.T) {
	acts, store, _, connID := setup(t)
	ctx := context.Background()
	seedUnit(t, store, connID)

	for _, ext := range []string{"art-a", "art-b"} {
		if err := store.Q.UpsertRawArtifact(ctx, gen.UpsertRawArtifactParams{
			TenantID: "tenant-y", Region: "us", ConnectionID: connID, Source: "github",
			SourceNativeKind: "issue", ContainerID: "acme/api", SyncContainerID: "acme/api",
			ExternalID: ext, ObjectKey: "bucket/" + ext, ContentHash: "h",
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Wrong tenant: must mark nothing (connection id is not a secret — tenant-scoped).
	if err := acts.MarkConnectionPurge(ctx, MarkPurgeInput{ConnectionID: connID, TenantID: "tenant-other"}); err != nil {
		t.Fatal(err)
	}
	if pend, _ := store.Q.ListPurgePendingArtifacts(ctx, 10); len(pend) != 0 {
		t.Fatalf("a cross-tenant disconnect must mark nothing, got %d", len(pend))
	}

	// Right tenant: both artifacts enter the outbox immediately.
	if err := acts.MarkConnectionPurge(ctx, MarkPurgeInput{ConnectionID: connID, TenantID: "tenant-y"}); err != nil {
		t.Fatal(err)
	}
	pend, err := store.Q.ListPurgePendingArtifacts(ctx, 10)
	if err != nil || len(pend) != 2 {
		t.Fatalf("disconnect must enqueue both artifacts, len=%d err=%v", len(pend), err)
	}
}
