package server

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
)

// Header names the API sets on the internal hop. The integrations service is
// internal-only and trusts these (design-doc 0009); service-to-service auth is a
// later hardening.
const (
	headerTenant = "Hivebook-Tenant-Id"
	headerRegion = "Hivebook-Region"
)

type ctxKey int

const (
	tenantKey ctxKey = iota
	regionKey
)

// tenantInterceptor lifts the tenant/region headers into the context for the
// user-facing methods — unary and streaming alike (WatchConnections needs it).
// CompleteConnection ignores them (it trusts the signed state), so this never
// rejects a request; handlers that need a tenant enforce it via requireTenant.
type tenantInterceptor struct{}

func newTenantInterceptor() connect.Interceptor { return tenantInterceptor{} }

func (tenantInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return next(withTenant(ctx, req.Header()), req)
	}
}

func (tenantInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // the integrations service is a stream server, not client
}

func (tenantInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(withTenant(ctx, conn.RequestHeader()), conn)
	}
}

func withTenant(ctx context.Context, h http.Header) context.Context {
	if t := h.Get(headerTenant); t != "" {
		ctx = context.WithValue(ctx, tenantKey, t)
	}
	if r := h.Get(headerRegion); r != "" {
		ctx = context.WithValue(ctx, regionKey, r)
	}
	return ctx
}

func tenantFrom(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tenantKey).(string)
	return t, ok && t != ""
}

func regionFrom(ctx context.Context) (string, bool) {
	r, ok := ctx.Value(regionKey).(string)
	return r, ok && r != ""
}

// requireTenant returns the caller's tenant or a CodeUnauthenticated error.
func requireTenant(ctx context.Context) (string, error) {
	t, ok := tenantFrom(ctx)
	if !ok {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("missing tenant"))
	}
	return t, nil
}
