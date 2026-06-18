// Package temporal wires the integrations service to the Temporal cluster: the
// client factory and the task-queue naming scheme. The orchestration itself
// lives in internal/workflow and internal/activity (design-doc 0012).
package temporal

import (
	"fmt"
	"log/slog"
	"os"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
)

// Defaults target the in-cluster dev server (infra/temporal). Override with the
// env vars below for `localhost` (port-forward) or a real cluster/namespace.
const (
	defaultHostPort  = "temporal-frontend.hivebook.svc.cluster.local:7233"
	defaultNamespace = "hivebook"

	envHostPort  = "HIVEBOOK_TEMPORAL_HOSTPORT"
	envNamespace = "HIVEBOOK_TEMPORAL_NAMESPACE"
)

// HostPort is the Temporal frontend address, env-overridable.
func HostPort() string {
	if v := os.Getenv(envHostPort); v != "" {
		return v
	}
	return defaultHostPort
}

// Namespace is the Temporal namespace, env-overridable. One namespace per region
// is the residency boundary (design-doc 0012); local dev runs a single namespace.
func Namespace() string {
	if v := os.Getenv(envNamespace); v != "" {
		return v
	}
	return defaultNamespace
}

// Dial builds a Temporal client from the environment. The caller owns Close(). The
// OTel tracing interceptor is always installed; it is a no-op unless HIVEBOOK_OTLP_ENDPOINT
// is set and InitTracing built a real provider (design-doc 0012, "Observability").
func Dial(logger *slog.Logger) (client.Client, error) {
	tracer, err := tracingInterceptor()
	if err != nil {
		return nil, fmt.Errorf("temporal tracing interceptor: %w", err)
	}
	c, err := client.Dial(client.Options{
		HostPort:     HostPort(),
		Namespace:    Namespace(),
		Logger:       newSlogLogger(logger),
		Interceptors: []interceptor.ClientInterceptor{tracer},
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal at %s (namespace %s): %w", HostPort(), Namespace(), err)
	}
	return c, nil
}

// ConnectionQueue carries the per-connection ConnectionWorkflow — the lifecycle
// driver. It holds no source I/O, so one queue serves every connector.
const ConnectionQueue = "connection"

// SyncTaskQueue is the per-connector sync queue (e.g. "sync-github"). Per-connector
// queues keep a throttled GitHub from starving Drive's worker slots (ADR-0034).
func SyncTaskQueue(connectorID string) string {
	return "sync-" + connectorID
}
