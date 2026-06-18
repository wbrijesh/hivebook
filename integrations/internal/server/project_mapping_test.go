package server

import (
	"database/sql"
	"testing"
	"time"

	"integrations/internal/database/gen"
)

// TestToProtoProjectMapsSyncUnit locks the sync_unit → proto Project mapping
// (design-doc 0012: a project IS a sync_unit). id is the unit's external_id,
// last_synced is its committed_at (ADR-0033), and a NULL committed_at leaves the
// timestamp unset — the Phase-5 repoint off the deleted project queries.
func TestToProtoProjectMapsSyncUnit(t *testing.T) {
	committed := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	row := gen.ListSyncUnitsRow{
		ExternalID:    "acme/api",
		Name:          "acme/api",
		Kind:          "repo",
		Selected:      true,
		Status:        "synced",
		LastError:     "",
		CommittedAt:   sql.NullTime{Time: committed, Valid: true},
		ArtifactCount: 7,
	}
	got := toProtoProject(row)
	if got.GetId() != "acme/api" {
		t.Fatalf("id = %q, want external_id acme/api", got.GetId())
	}
	if got.GetArtifactCount() != 7 {
		t.Fatalf("artifact_count = %d, want 7", got.GetArtifactCount())
	}
	if !got.GetLastSyncedAt().AsTime().Equal(committed) {
		t.Fatalf("last_synced_at = %v, want committed_at %v", got.GetLastSyncedAt().AsTime(), committed)
	}

	// A never-committed unit leaves last_synced unset.
	uncommitted := toProtoProject(gen.ListSyncUnitsRow{ExternalID: "acme/web", Status: "idle"})
	if uncommitted.GetLastSyncedAt() != nil {
		t.Fatalf("uncommitted unit must have nil last_synced_at, got %v", uncommitted.GetLastSyncedAt())
	}
}
