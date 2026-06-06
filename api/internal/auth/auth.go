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
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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
	verifier atomic.Pointer[oidc.IDTokenVerifier]
}

// New starts an Authenticator and kicks off provider discovery in the
// background. caFile, if set, is appended to the system cert pool so the issuer's
// mkcert-signed TLS is trusted in-cluster.
func New(issuer, audience, caFile string) *Authenticator {
	a := &Authenticator{issuer: issuer, audience: audience}
	go a.discover(buildHTTPClient(caFile))
	return a
}

// Ready reports whether provider discovery has completed.
func (a *Authenticator) Ready() bool { return a.verifier.Load() != nil }

func (a *Authenticator) discover(hc *http.Client) {
	for {
		ctx := oidc.ClientContext(context.Background(), hc)
		provider, err := oidc.NewProvider(ctx, a.issuer)
		if err != nil {
			log.Printf("auth: OIDC discovery for %s failed, retrying: %v", a.issuer, err)
			time.Sleep(5 * time.Second)
			continue
		}
		// Audience is set once the ZITADEL api app exists (Phase 3). Until then
		// we still verify signature, issuer and expiry, just not the audience.
		cfg := &oidc.Config{ClientID: a.audience}
		if a.audience == "" {
			cfg.SkipClientIDCheck = true
			log.Printf("auth: no HIVEBOOK_OIDC_AUDIENCE set — skipping audience check")
		}
		a.verifier.Store(provider.Verifier(cfg))
		log.Printf("auth: OIDC provider ready (issuer=%s audience=%q)", a.issuer, a.audience)
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
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		tok, err := v.Verify(r.Context(), raw)
		if err != nil {
			verifications.WithLabelValues("invalid").Inc()
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
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
			log.Printf("auth: could not read OIDC CA file %s: %v", caFile, err)
		}
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
}
