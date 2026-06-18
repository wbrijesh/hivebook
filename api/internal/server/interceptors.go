package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5/middleware"

	"api/internal/auth"
)

// sanitizeInterceptor keeps internal detail from leaking to the browser. It is
// default-DENY: only explicitly client-meaningful codes — which our handlers /
// protovalidate construct with intentional, safe messages the frontend acts on —
// pass through unchanged; every other code (Internal, Unknown, DataLoss,
// Unavailable, Aborted, DeadlineExceeded, Canceled, …) is replaced with a generic
// message (code preserved) before leaving the process, with the original logged.
// A whitelist, not a blacklist, so a future handler that surfaces a raw error under
// an unanticipated code can't leak it (design-doc 0011 P3-5, standards/go/errors).
// Outermost so it covers handler, proxy, auth, and validation errors alike.
type sanitizeInterceptor struct{}

func newSanitizeInterceptor() connect.Interceptor { return sanitizeInterceptor{} }

// clientSafeCodes carry messages meant for the caller (no internal detail). Every
// other code is scrubbed.
var clientSafeCodes = map[connect.Code]bool{
	connect.CodeInvalidArgument:    true,
	connect.CodeNotFound:           true,
	connect.CodeAlreadyExists:      true,
	connect.CodePermissionDenied:   true,
	connect.CodeUnauthenticated:    true,
	connect.CodeFailedPrecondition: true,
	connect.CodeResourceExhausted:  true,
	connect.CodeOutOfRange:         true,
	connect.CodeUnimplemented:      true,
}

func sanitizeError(ctx context.Context, proc string, err error) error {
	if err == nil {
		return nil
	}
	code := connect.CodeOf(err)
	if clientSafeCodes[code] {
		return err
	}
	slog.Error("rpc_error", "procedure", proc, "code", code.String(),
		"error", err.Error(), "request_id", middleware.GetReqID(ctx))
	msg := "internal error"
	if code == connect.CodeUnavailable || code == connect.CodeDeadlineExceeded {
		msg = "service temporarily unavailable"
	}
	return connect.NewError(code, errors.New(msg))
}

func (sanitizeInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		res, err := next(ctx, req)
		if err != nil {
			return res, sanitizeError(ctx, req.Spec().Procedure, err)
		}
		return res, nil
	}
}

// WrapStreamingClient is the API acting as a client to the integrations service —
// an internal hop, not a browser response — so it isn't sanitized.
func (sanitizeInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (sanitizeInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return sanitizeError(ctx, conn.Spec().Procedure, next(ctx, conn))
	}
}

// metricsInterceptor records per-procedure RPC count, result code, and latency —
// for unary calls and streaming alike (WatchConnections). The HTTP middleware
// sees only the collapsed wildcard route for the Connect mount, so this is the
// source of per-method observability. Placed outermost so it times the whole RPC
// and records auth/validation failures too.
type metricsInterceptor struct{}

func newMetricsInterceptor() connect.Interceptor { return metricsInterceptor{} }

func (metricsInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		start := time.Now()
		res, err := next(ctx, req)
		observeRPC(req.Spec().Procedure, err, start)
		return res, err
	}
}

func (metricsInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (metricsInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		start := time.Now()
		err := next(ctx, conn)
		observeRPC(conn.Spec().Procedure, err, start)
		return err
	}
}

func observeRPC(proc string, err error, start time.Time) {
	code := "ok"
	if err != nil {
		code = connect.CodeOf(err).String()
	}
	rpcRequests.WithLabelValues(proc, code).Inc()
	rpcDuration.WithLabelValues(proc).Observe(time.Since(start).Seconds())
}

// authenticator is the slice of the auth layer the interceptor needs: verify a
// bearer header and resolve the caller. *auth.Authenticator satisfies it; a stub
// satisfies it in tests.
type authenticator interface {
	Authenticate(ctx context.Context, authzHeader string) (auth.Identity, error)
}

// authInterceptor verifies the caller on every RPC — unary and streaming — and
// puts the resolved identity in the context for handlers (auth.IdentityFromContext).
// Failures map to Connect codes: provider-not-ready is transient (CodeUnavailable);
// everything else — missing/invalid token, no resolvable org — is
// CodeUnauthenticated. No token detail leaks to the client (standards/go/errors).
type authInterceptor struct{ a authenticator }

func newAuthInterceptor(a authenticator) connect.Interceptor { return authInterceptor{a: a} }

func (i authInterceptor) authError(err error) error {
	if errors.Is(err, auth.ErrProviderNotReady) {
		return connect.NewError(connect.CodeUnavailable, errors.New("auth provider not ready"))
	}
	return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
}

func (i authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		id, err := i.a.Authenticate(ctx, req.Header().Get("Authorization"))
		if err != nil {
			return nil, i.authError(err)
		}
		return next(auth.ContextWithIdentity(ctx, id), req)
	}
}

func (i authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		id, err := i.a.Authenticate(ctx, conn.RequestHeader().Get("Authorization"))
		if err != nil {
			return i.authError(err)
		}
		return next(auth.ContextWithIdentity(ctx, id), conn)
	}
}
