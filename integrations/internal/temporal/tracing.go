package temporal

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	otelinterceptor "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
)

// envOTLPEndpoint gates tracing. Set it (e.g. "otel-collector.hivebook.svc:4317") and
// spans export over OTLP/gRPC; leave it unset and tracing is a no-op. This is the
// "ready in production, free in dev" switch design-doc 0012 asks for: the interceptor
// is always wired into both clients, but with no endpoint it uses the global no-op
// tracer and costs nothing.
const envOTLPEndpoint = "HIVEBOOK_OTLP_ENDPOINT"

// tracingInterceptor builds the Temporal OTel tracing interceptor. It always uses the
// global tracer provider — InitTracing swaps that for a real OTLP provider when the
// endpoint is set, otherwise it stays the SDK's no-op. Wired into BOTH the worker and
// the server clients via client.Options.Interceptors so a sync's trace spans the RPC,
// the workflow, and its activities (the absence of which made the old debugging
// archaeology — design-doc 0012, "Observability").
func tracingInterceptor() (interceptor.Interceptor, error) {
	return otelinterceptor.NewTracingInterceptor(otelinterceptor.TracerOptions{
		Tracer: otel.Tracer("hivebook-integrations"),
	})
}

// InitTracing installs a tracer provider gated on HIVEBOOK_OTLP_ENDPOINT and returns a
// shutdown func (always non-nil). With the endpoint set it builds an OTLP/gRPC exporter
// to that endpoint and registers it as the global provider; unset, it is a no-op and
// the returned shutdown does nothing. Call once at process start (worker + server) and
// defer the shutdown so spans flush on exit.
func InitTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := os.Getenv(envOTLPEndpoint)
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(), // in-cluster collector; TLS is the mesh's job
	)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter (%s): %w", endpoint, err)
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
	))
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
