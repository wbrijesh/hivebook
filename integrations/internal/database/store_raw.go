package database

import "context"

// NotifyConnectionChanged fires a Postgres NOTIFY on the connection_changed
// channel with the tenant as payload. The server's LISTEN loop fans this out to
// the tenant's WatchConnections streams — cheap, event-driven push (no polling).
func (s *Store) NotifyConnectionChanged(ctx context.Context, tenant string) error {
	_, err := s.db.ExecContext(ctx, `SELECT pg_notify('connection_changed', $1)`, tenant)
	return err
}
