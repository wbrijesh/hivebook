// Package auth verifies ZITADEL-issued OIDC access tokens (JWT) on protected
// routes. Verification is offline: we fetch the issuer's discovery document and
// JWKS once, then validate each bearer token's signature, issuer, expiry, and
// audience locally.
//
// Two wrinkles specific to our setup are handled here:
//   - The issuer (https://id.hivebook.localhost) is served with a cert signed by
//     the local mkcert CA, so we add that CA to the HTTP client's trust pool.
//   - ZITADEL may not be reachable the instant the API starts, so provider
//     discovery runs in the background with retries; until it succeeds the
//     middleware returns 503 rather than failing the whole server.
package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/oauth2"
)

// verifications counts token-verification outcomes on protected routes.
var verifications = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "hivebook_auth_verifications_total",
	Help: "Token verification outcomes on protected routes.",
}, []string{"result"})

type ctxKey int

const claimsKey ctxKey = iota

// Authenticator holds the lazily-initialised token verifier.
type Authenticator struct {
	issuer   string
	audience string
	hc       *http.Client
	idCache  *identityCache
	provider atomic.Pointer[oidc.Provider]
	verifier atomic.Pointer[oidc.IDTokenVerifier]
}

// Identity cache sizing (constants-first, ADR-0011). Bounded so the process
// can't grow without limit; the TTL re-resolves a user after org changes.
const (
	identityCacheSize = 4096
	identityCacheTTL  = 30 * time.Minute
	// OIDC discovery retry backoff grows from 1s up to this cap.
	maxDiscoveryBackoff = 60 * time.Second
)

// New starts an Authenticator and kicks off provider discovery in the
// background. caFile, if set, is appended to the system cert pool so the issuer's
// mkcert-signed TLS is trusted in-cluster.
func New(issuer, audience, caFile string) *Authenticator {
	a := &Authenticator{
		issuer:   issuer,
		audience: audience,
		hc:       buildHTTPClient(caFile),
		idCache:  newIdentityCache(identityCacheSize, identityCacheTTL),
	}
	go a.discover(a.hc)
	return a
}

// Ready reports whether provider discovery has completed.
func (a *Authenticator) Ready() bool { return a.verifier.Load() != nil }

func (a *Authenticator) discover(hc *http.Client) {
	backoff := time.Second
	for {
		ctx := oidc.ClientContext(context.Background(), hc)
		provider, err := oidc.NewProvider(ctx, a.issuer)
		if err != nil {
			slog.Warn("oidc_discovery_retry", "issuer", a.issuer, "error", err.Error(), "backoff", backoff.String())
			time.Sleep(backoff)
			if backoff *= 2; backoff > maxDiscoveryBackoff {
				backoff = maxDiscoveryBackoff
			}
			continue
		}
		// Audience is set once the ZITADEL api app exists (Phase 3). Until then
		// we still verify signature, issuer and expiry, just not the audience.
		cfg := &oidc.Config{ClientID: a.audience}
		if a.audience == "" {
			cfg.SkipClientIDCheck = true
			slog.Warn("oidc_audience_check_skipped", "issuer", a.issuer)
		}
		a.provider.Store(provider)
		a.verifier.Store(provider.Verifier(cfg))
		slog.Info("oidc_provider_ready", "issuer", a.issuer, "audience", a.audience)
		return
	}
}

// Middleware verifies the bearer access token and stores its claims in the
// request context. Unauthenticated or invalid requests are rejected.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := a.verifier.Load()
		if v == nil {
			verifications.WithLabelValues("not_ready").Inc()
			http.Error(w, "auth provider not ready", http.StatusServiceUnavailable)
			return
		}
		raw, err := bearerToken(r)
		if err != nil {
			verifications.WithLabelValues("missing").Inc()
			http.Error(w, "missing or malformed authorization header", http.StatusUnauthorized)
			return
		}
		tok, err := v.Verify(r.Context(), raw)
		if err != nil {
			verifications.WithLabelValues("invalid").Inc()
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		var claims map[string]any
		if err := tok.Claims(&claims); err != nil {
			verifications.WithLabelValues("invalid").Inc()
			http.Error(w, "cannot parse claims", http.StatusUnauthorized)
			return
		}
		verifications.WithLabelValues("ok").Inc()
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Claims returns the verified token claims stored by Middleware.
func Claims(r *http.Request) (map[string]any, bool) {
	c, ok := r.Context().Value(claimsKey).(map[string]any)
	return c, ok
}

// Identity is the caller's resolved profile: who they are (sub/name/email) and
// which ZITADEL organization owns them (our tenant id, design-doc 0005).
type Identity struct {
	Sub   string
	Name  string
	Email string
	Org   string
}

func strClaim(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func orgFromClaims(m map[string]any) string {
	for _, k := range []string{
		"urn:zitadel:iam:user:resourceowner:id",
		"urn:zitadel:iam:org:id",
	} {
		if v := strClaim(m, k); v != "" {
			return v
		}
	}
	return ""
}

func nameFromClaims(m map[string]any) string {
	if v := strClaim(m, "name"); v != "" {
		return v
	}
	return strClaim(m, "preferred_username")
}

// Identity resolves the caller's profile and organization. It prefers the token
// claims, and otherwise fetches them from the userinfo endpoint — ZITADEL's
// access tokens are minimal, exposing name/email and the resource-owner org only
// at userinfo (and only with the scopes the web app requests). Cached per user.
func (a *Authenticator) Identity(ctx context.Context, r *http.Request) (Identity, error) {
	claims, _ := Claims(r)
	sub, _ := claims["sub"].(string)

	if sub != "" {
		if id, ok := a.idCache.get(sub); ok {
			return id, nil
		}
	}

	id := Identity{
		Sub:   sub,
		Name:  nameFromClaims(claims),
		Email: strClaim(claims, "email"),
		Org:   orgFromClaims(claims),
	}

	// Fill any gaps from userinfo (one call covers org, name and email).
	if id.Org == "" || id.Name == "" || id.Email == "" {
		info, err := a.userInfo(ctx, r)
		if err != nil {
			return Identity{}, err
		}
		if id.Org == "" {
			id.Org = orgFromClaims(info)
		}
		if id.Name == "" {
			id.Name = nameFromClaims(info)
		}
		if id.Email == "" {
			id.Email = strClaim(info, "email")
		}
	}

	if id.Org == "" {
		return Identity{}, errors.New("no organization in userinfo (missing resourceowner scope?)")
	}
	if sub != "" {
		a.idCache.add(sub, id)
	}
	return id, nil
}

// userInfo calls the issuer's userinfo endpoint with the request's bearer token.
func (a *Authenticator) userInfo(ctx context.Context, r *http.Request) (map[string]any, error) {
	p := a.provider.Load()
	if p == nil {
		return nil, errors.New("auth provider not ready")
	}
	raw, err := bearerToken(r)
	if err != nil {
		return nil, err
	}
	ui, err := p.UserInfo(oidc.ClientContext(ctx, a.hc),
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: raw}))
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	var info map[string]any
	if err := ui.Claims(&info); err != nil {
		return nil, err
	}
	return info, nil
}

func bearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("malformed Authorization header")
	}
	return parts[1], nil
}

func buildHTTPClient(caFile string) *http.Client {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if caFile != "" {
		if pem, err := os.ReadFile(caFile); err == nil {
			pool.AppendCertsFromPEM(pem)
		} else {
			slog.Warn("oidc_ca_file_unreadable", "path", caFile, "error", err.Error())
		}
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
}
