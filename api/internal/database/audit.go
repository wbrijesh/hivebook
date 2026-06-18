package database

import (
	"context"
	"database/sql"
	"time"

	"api/internal/database/gen"
)

// AuditEvent is one recorded privileged action in a workspace (ADR-0010).
type AuditEvent struct {
	ID         string
	OccurredAt time.Time
	ActorID    string
	ActorEmail string // empty when unknown
	Action     string
	Target     string // empty when not applicable
}

// RecordAuditEvent appends an event to the tenant's trail, resolving the tenant
// by its ZITADEL org id so callers only need the identity they already hold.
// Best-effort by convention — callers log and continue rather than failing the
// user's action on an audit-write error.
func (s *service) RecordAuditEvent(ctx context.Context, orgID, actorID, actorEmail, action, target string) error {
	t, err := s.q.GetTenantByOrg(ctx, orgID)
	if err != nil {
		return err
	}
	return s.q.RecordAuditEvent(ctx, gen.RecordAuditEventParams{
		TenantID:   t.ID,
		ActorID:    actorID,
		ActorEmail: nullStr(actorEmail),
		Action:     action,
		Target:     nullStr(target),
		Metadata:   []byte("{}"),
	})
}

// ListAuditEvents returns a page of the tenant's trail, newest first, plus the
// total row count for pagination.
func (s *service) ListAuditEvents(ctx context.Context, orgID string, limit, offset int32) ([]AuditEvent, int32, error) {
	t, err := s.q.GetTenantByOrg(ctx, orgID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAuditEvents(ctx, gen.ListAuditEventsParams{
		TenantID:  t.ID,
		RowLimit:  limit,
		RowOffset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountAuditEvents(ctx, t.ID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AuditEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, AuditEvent{
			ID:         r.ID,
			OccurredAt: r.CreatedAt,
			ActorID:    r.ActorID,
			ActorEmail: r.ActorEmail.String,
			Action:     r.Action,
			Target:     r.Target.String,
		})
	}
	return out, int32(total), nil
}

// nullStr maps an empty string to SQL NULL — empty actor email / target read as
// absent, not as the empty string.
func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
