package server

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	"api/internal/auth"
)

// newMetricsInterceptor records per-procedure RPC count, result code, and
// latency. The HTTP middleware sees only the collapsed wildcard route for the
// Connect mount, so this is the source of per-method observability. Placed
// outermost so it times the whole RPC and records auth/validation failures too.
func newMetricsInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			res, err := next(ctx, req)
			proc := req.Spec().Procedure
			code := "ok"
			if err != nil {
				code = connect.CodeOf(err).String()
			}
			rpcRequests.WithLabelValues(proc, code).Inc()
			rpcDuration.WithLabelValues(proc).Observe(time.Since(start).Seconds())
			return res, err
		}
	}
}

// authenticator is the slice of the auth layer the interceptor needs: verify a
// bearer header and resolve the caller. *auth.Authenticator satisfies it; a stub
// satisfies it in tests.
type authenticator interface {
	Authenticate(ctx context.Context, authzHeader string) (auth.Identity, error)
}

// newAuthInterceptor verifies the caller on every RPC and puts the resolved
// identity in the context for handlers (auth.IdentityFromContext). Failures map
// to Connect codes: provider-not-ready is transient (CodeUnavailable);
// everything else — missing/invalid token, no resolvable org — is
// CodeUnauthenticated. No token detail leaks to the client (standards/go/errors).
func newAuthInterceptor(a authenticator) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			id, err := a.Authenticate(ctx, req.Header().Get("Authorization"))
			if err != nil {
				if errors.Is(err, auth.ErrProviderNotReady) {
					return nil, connect.NewError(connect.CodeUnavailable,
						errors.New("auth provider not ready"))
				}
				return nil, connect.NewError(connect.CodeUnauthenticated,
					errors.New("authentication required"))
			}
			return next(auth.ContextWithIdentity(ctx, id), req)
		}
	}
}
