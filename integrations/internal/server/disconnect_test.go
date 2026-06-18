package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	"integrations/internal/database/gen"
	integrationv1 "integrations/internal/gen/hivebook/integration/v1"
)

// TestDisconnectRejectsCrossTenant is the handler-level lock for the P0
// tenant-isolation fix (design-doc 0011): a connection id is not a secret, so a
// caller in another tenant must get NotFound and trigger no destructive action —
// the connection survives. The store-level test proves the SQL is scoped; this
// proves the handler proves ownership (GetConnection) before anything destructive.
func TestDisconnectRejectsCrossTenant(t *testing.T) {
	s := newStore(t)
	srv := &Server{store: s} // Disconnect's authz path uses only the store + creds;
	// creds is nil, so the GitHub-uninstall block (reached only after ownership is
	// proven) is a no-op — exactly the "no uninstall attempted" property we want.
	ctx := context.Background()

	connA, err := s.Q.UpsertConnection(ctx, gen.UpsertConnectionParams{
		TenantID: "org_A", Region: "us-east", ConnectorID: "github_pat",
		Account: "acct", AccessToken: []byte("a"), RefreshToken: []byte("r"),
		ExternalID: "acct",
	})
	if err != nil {
		t.Fatalf("seed connection: %v", err)
	}

	// Tenant B attempts to disconnect tenant A's connection → NotFound, no effect.
	ctxB := context.WithValue(ctx, tenantKey, "org_B")
	_, err = srv.Disconnect(ctxB, connect.NewRequest(&integrationv1.DisconnectRequest{ConnectionId: connA.ID}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("cross-tenant Disconnect must be NotFound, got %v", err)
	}
	got, err := s.Q.GetConnection(ctx, gen.GetConnectionParams{ID: connA.ID, TenantID: "org_A"})
	if err != nil {
		t.Fatalf("tenant A's connection must survive a cross-tenant disconnect: %v", err)
	}
	if got.Status == "disabled" {
		t.Fatal("tenant A's connection must not be disabled by tenant B")
	}

	// The owner can disconnect (proves the NotFound above is authz, not a dead path).
	ctxA := context.WithValue(ctx, tenantKey, "org_A")
	if _, err := srv.Disconnect(ctxA, connect.NewRequest(&integrationv1.DisconnectRequest{ConnectionId: connA.ID})); err != nil {
		t.Fatalf("owner Disconnect should succeed: %v", err)
	}
	if _, err := s.Q.GetConnection(ctx, gen.GetConnectionParams{ID: connA.ID, TenantID: "org_A"}); err == nil {
		t.Fatal("owner Disconnect should soft-delete the connection (GetConnection now misses)")
	}
}
