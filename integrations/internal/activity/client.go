package activity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"integrations/internal/connectors"
	"integrations/internal/database/gen"
)

// refreshSkew is how far before the stored expiry we proactively refresh, so a token
// doesn't expire mid-drain. An invalid/zero expiry (a PAT, or a token without an expiry)
// is treated as non-expiring and never refreshed.
const refreshSkew = 2 * time.Minute

// refreshLocks single-flights token refresh per connection: concurrent SyncUnit
// activities for one connection share one refresh instead of racing to rotate the token
// and clobbering each other's write. Process-level — Temporal may run several of a
// connection's units on one worker. Keyed by connection id.
var refreshLocks sync.Map // connection id -> *sync.Mutex

func connLock(id string) *sync.Mutex {
	m, _ := refreshLocks.LoadOrStore(id, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// bearerRoundTripper attaches a static bearer token to every request. The token is fixed
// for the activity's lifetime; the refresh decision happens once, in clientFor, before
// the client is built (single-flight per connection), so the round-tripper stays simple.
type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", "Bearer "+b.token)
	base := b.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

// clientFor builds the HTTP client a sync activity uses to call a source:
//
//   - GitHub App installation: mint a short-lived installation token from the App
//     private key (installation tokens are re-minted each activity; nothing to persist).
//   - OAuth connection: refresh the stored access token when it is expired or near
//     expiry via the connector's OAuthConfig().TokenSource, persist the rotated token,
//     and hand back a client over the live token. The refresh is single-flight per
//     connection (refreshLocks) so concurrent units don't clobber each other's rotation.
//   - PAT / no-expiry token: a static bearer over the stored token, no refresh.
//
// note: no secrets ever enter workflow state — this is all activity-side. A refresh that
// fails with invalid_grant returns a connectors.AuthError so the caller (classify) flips
// the connection to needs_reauth (non-retryable); the credentials are dead, not throttled.
func (a *Activities) clientFor(ctx context.Context, conn gen.ConnectorConnection) (*http.Client, error) {
	creds := a.creds[conn.ConnectorID]

	// GitHub App: mint a short-lived installation token from the App private key.
	if conn.InstallationID != "" && creds.PrivateKey != "" {
		key, err := connectors.ParseGitHubKey([]byte(creds.PrivateKey))
		if err != nil {
			return nil, err
		}
		token, _, err := connectors.GitHubInstallationToken(ctx, http.DefaultClient, creds.ClientID, key, conn.InstallationID)
		if err != nil {
			return nil, err
		}
		return &http.Client{Transport: bearerRoundTripper{token: token}}, nil
	}

	token, err := a.refreshedAccessToken(ctx, creds, conn)
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: bearerRoundTripper{token: token}}, nil
}

// refreshedAccessToken returns a live access token for the connection, rotating and
// persisting it first if it is expired or near expiry. It is single-flight per
// connection. When no refresh is possible (no refresh token, or no expiry to act on) it
// returns the stored access token unchanged.
func (a *Activities) refreshedAccessToken(ctx context.Context, creds connectors.Creds, conn gen.ConnectorConnection) (string, error) {
	access, err := a.cipher.DecryptString(conn.AccessToken)
	if err != nil {
		return "", err
	}
	refresh, err := a.cipher.DecryptString(conn.RefreshToken)
	if err != nil {
		return "", err
	}

	// Nothing to refresh with, or no expiry signalling staleness → use the token as-is.
	if refresh == "" || !conn.TokenExpiry.Valid {
		return access, nil
	}
	if time.Until(conn.TokenExpiry.Time) > refreshSkew {
		return access, nil // still fresh
	}

	// Single-flight: one refresh per connection. The loser of the lock re-reads the row so
	// it picks up the winner's freshly-rotated token instead of refreshing again.
	mu := connLock(conn.ID)
	mu.Lock()
	defer mu.Unlock()

	if cur, err := a.store.Q.GetConnectionByID(ctx, conn.ID); err == nil {
		conn = cur
		if access2, err := a.cipher.DecryptString(conn.AccessToken); err == nil {
			access = access2
		}
		if refresh2, err := a.cipher.DecryptString(conn.RefreshToken); err == nil && refresh2 != "" {
			refresh = refresh2
		}
		if conn.TokenExpiry.Valid && time.Until(conn.TokenExpiry.Time) > refreshSkew {
			return access, nil // another unit already refreshed while we waited
		}
	}

	connector, ok := a.reg.Get(conn.ConnectorID)
	if !ok {
		return "", fmt.Errorf("unknown connector %q", conn.ConnectorID)
	}
	cfg := connector.OAuthConfig(creds.ClientID, creds.ClientSecret, "")
	src := cfg.TokenSource(ctx, &oauth2.Token{
		AccessToken:  access,
		RefreshToken: refresh,
		Expiry:       conn.TokenExpiry.Time,
	})
	tok, err := src.Token()
	if err != nil {
		if isInvalidGrant(err) {
			// The refresh token is dead — terminal credentials failure, not a transient
			// error. Surface as AuthError so classify flips to needs_reauth (non-retryable).
			return "", &connectors.AuthError{Status: http.StatusUnauthorized}
		}
		return "", err
	}

	if err := a.persistRotatedToken(ctx, conn.ID, tok); err != nil {
		return "", err
	}
	return tok.AccessToken, nil
}

// persistRotatedToken writes the rotated token back. It NEVER blanks a good refresh
// token: when the provider returns an empty refresh token (reuse), it uses
// UpdateAccessToken so the stored refresh token survives; only a non-empty rotation
// writes both via UpdateTokens (design-doc 0010/0011).
func (a *Activities) persistRotatedToken(ctx context.Context, connID string, tok *oauth2.Token) error {
	encAccess, err := a.cipher.EncryptString(tok.AccessToken)
	if err != nil {
		return err
	}
	expiry := nullTimeVal(tok.Expiry)
	if tok.RefreshToken == "" {
		return a.store.Q.UpdateAccessToken(ctx, gen.UpdateAccessTokenParams{
			AccessToken: encAccess, TokenExpiry: expiry, ID: connID,
		})
	}
	encRefresh, err := a.cipher.EncryptString(tok.RefreshToken)
	if err != nil {
		return err
	}
	return a.store.Q.UpdateTokens(ctx, gen.UpdateTokensParams{
		AccessToken: encAccess, RefreshToken: encRefresh, TokenExpiry: expiry, ID: connID,
	})
}

// isInvalidGrant reports whether an OAuth refresh failed because the refresh token is
// revoked/expired (RFC 6749 invalid_grant) — terminal, vs a transient network/5xx error.
func isInvalidGrant(err error) bool {
	var re *oauth2.RetrieveError
	if errors.As(err, &re) {
		if strings.Contains(string(re.Body), "invalid_grant") {
			return true
		}
		return re.Response != nil && re.Response.StatusCode == http.StatusBadRequest
	}
	return strings.Contains(err.Error(), "invalid_grant")
}
