package activity

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"integrations/internal/database/gen"
)

// TestPersistRotatedTokenPreservesRefresh locks the never-blank-a-good-refresh-token
// rule (design-doc 0010/0011): a rotation that returns a refresh token writes both
// (UpdateTokens); a rotation that omits it writes only the access token
// (UpdateAccessToken) so the stored refresh token survives the next refresh.
func TestPersistRotatedTokenPreservesRefresh(t *testing.T) {
	acts, store, _, connID := setup(t)
	ctx := context.Background()

	// Seed a stored refresh token so we can prove it survives a refresh-token-less rotation.
	encRefresh, _ := acts.cipher.EncryptString("stored-refresh")
	if err := store.Q.UpdateTokens(ctx, gen.UpdateTokensParams{
		AccessToken:  mustEnc(t, acts, "old-access"),
		RefreshToken: encRefresh,
		TokenExpiry:  nullTimeVal(time.Now().Add(time.Hour)),
		ID:           connID,
	}); err != nil {
		t.Fatal(err)
	}

	// Rotation WITHOUT a refresh token → UpdateAccessToken → refresh token preserved.
	if err := acts.persistRotatedToken(ctx, connID, &oauth2.Token{
		AccessToken: "new-access", Expiry: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	conn, _ := store.Q.GetConnectionByID(ctx, connID)
	if got, _ := acts.cipher.DecryptString(conn.AccessToken); got != "new-access" {
		t.Fatalf("access token should rotate, got %q", got)
	}
	if got, _ := acts.cipher.DecryptString(conn.RefreshToken); got != "stored-refresh" {
		t.Fatalf("a refresh-token-less rotation must preserve the stored refresh token, got %q", got)
	}

	// Rotation WITH a refresh token → UpdateTokens → both rotate.
	if err := acts.persistRotatedToken(ctx, connID, &oauth2.Token{
		AccessToken: "newer-access", RefreshToken: "rotated-refresh", Expiry: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	conn, _ = store.Q.GetConnectionByID(ctx, connID)
	if got, _ := acts.cipher.DecryptString(conn.RefreshToken); got != "rotated-refresh" {
		t.Fatalf("a rotation that returns a refresh token must persist it, got %q", got)
	}
}

// TestRefreshedAccessTokenSkipsFreshAndPATs locks the no-op paths: a non-expiring token
// (no expiry, a PAT) and a still-fresh token are returned as-is without a refresh round
// trip — only an expired/near-expiry OAuth token with a refresh token rotates.
func TestRefreshedAccessTokenSkipsFreshAndPATs(t *testing.T) {
	acts, store, _, connID := setup(t)
	ctx := context.Background()

	// A static bearer (no refresh token, no expiry) → returned unchanged, no refresh.
	if err := store.Q.UpdateAccessToken(ctx, gen.UpdateAccessTokenParams{
		AccessToken: mustEnc(t, acts, "pat"), TokenExpiry: nullTimeVal(time.Time{}), ID: connID,
	}); err != nil {
		t.Fatal(err)
	}
	conn, _ := store.Q.GetConnectionByID(ctx, connID)
	tok, err := acts.refreshedAccessToken(ctx, acts.creds[conn.ConnectorID], conn)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "pat" {
		t.Fatalf("a PAT must be returned as-is, got %q", tok)
	}

	// A fresh OAuth token (expiry far out) with a refresh token → not refreshed.
	encRefresh, _ := acts.cipher.EncryptString("rt")
	if err := store.Q.UpdateTokens(ctx, gen.UpdateTokensParams{
		AccessToken:  mustEnc(t, acts, "fresh-access"),
		RefreshToken: encRefresh,
		TokenExpiry:  nullTimeVal(time.Now().Add(time.Hour)),
		ID:           connID,
	}); err != nil {
		t.Fatal(err)
	}
	conn, _ = store.Q.GetConnectionByID(ctx, connID)
	tok, err = acts.refreshedAccessToken(ctx, acts.creds[conn.ConnectorID], conn)
	if err != nil {
		t.Fatal(err)
	}
	if tok != "fresh-access" {
		t.Fatalf("a fresh token must not be refreshed, got %q", tok)
	}
}

// TestIsInvalidGrant locks the terminal-vs-transient classification driving needs_reauth:
// an invalid_grant (body or 400) is terminal; a 5xx / transport error is not.
func TestIsInvalidGrant(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"invalid_grant body", &oauth2.RetrieveError{Body: []byte(`{"error":"invalid_grant"}`)}, true},
		{"400 no body", &oauth2.RetrieveError{Response: &http.Response{StatusCode: 400}}, true},
		{"500 transient", &oauth2.RetrieveError{Response: &http.Response{StatusCode: 500}, Body: []byte("boom")}, false},
		{"plain error", errors.New("dial tcp: timeout"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isInvalidGrant(tc.err); got != tc.want {
				t.Fatalf("isInvalidGrant(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func mustEnc(t *testing.T, a *Activities, s string) []byte {
	t.Helper()
	b, err := a.cipher.EncryptString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
