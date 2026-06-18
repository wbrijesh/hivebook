// Command worker runs the Temporal sync worker (design-doc 0012, ADR-0034). It
// hosts the ConnectionWorkflow — the per-connection lifecycle driver — and the
// connector sync activities that do the source I/O. The old River worker is gone
// (Phase 0); this is its replacement.
//
// Scale-to-zero is safe: durable timers and state live in Temporal, not here, so a
// due sync stays pollable regardless of whether any worker exists, and KEDA scales
// on real task-queue backlog.
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.temporal.io/sdk/worker"

	"integrations/internal/activity"
	"integrations/internal/connectors"
	"integrations/internal/database"
	"integrations/internal/oauth"
	"integrations/internal/storage"
	"integrations/internal/temporal"
	"integrations/internal/workflow"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("worker_init_failed", "error", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx := context.Background()

	// --- Real dependencies (mirrors cmd/integrations) ------------------------
	store, err := database.New(database.ConfigFromEnv())
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer store.Close()

	tokenKey, err := base64.StdEncoding.DecodeString(os.Getenv("HIVEBOOK_INTEGRATIONS_TOKEN_KEY"))
	if err != nil {
		return fmt.Errorf("token key (base64): %w", err)
	}
	cipher, err := oauth.NewCipher(tokenKey)
	if err != nil {
		return fmt.Errorf("token cipher: %w", err)
	}

	st, err := storage.New(ctx, storage.ConfigFromEnv())
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	reg := connectors.NewRegistry(connectors.NewGoogleDocs(), connectors.NewGitHub(), connectors.NewGitHubPAT())
	creds := connectors.CredsFromEnv(reg)

	acts := activity.New(store, st, reg, cipher, creds)

	// --- OTel tracing (gated on HIVEBOOK_OTLP_ENDPOINT, no-op unset) ----------
	shutdownTracing, err := temporal.InitTracing(ctx, "integrations-worker")
	if err != nil {
		return fmt.Errorf("tracing: %w", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutCtx)
	}()

	// --- Temporal workers ----------------------------------------------------
	c, err := temporal.Dial(logger)
	if err != nil {
		return err
	}
	defer c.Close()

	// Queue → worker mapping (ADR-0034, design-doc 0012). A Temporal worker polls exactly
	// ONE task queue, so we run several worker instances in this one process:
	//
	//   ConnectionQueue ("connection")  — the ConnectionWorkflow + MaintenanceWorkflow and
	//                                     the connector-agnostic CONTROL activities
	//                                     (StartRun, DiscoverUnits, NextUnitBatch,
	//                                     SettleConnection, Capacity, the erasure/GC sweep).
	//   sync-<connector> (per connector) — the SyncUnit source-I/O activity ONLY.
	//
	// The split is the isolation guarantee: a throttled GitHub's SyncUnit tasks pile up on
	// sync-github and can't starve Drive's sync-gdocs slots, while the lifecycle workflow
	// keeps driving on ConnectionQueue regardless. The workflow routes SyncUnit to the
	// per-connector queue via ActivityOptions.TaskQueue (internal/workflow/connection.go).
	var workers []worker.Worker

	connW := worker.New(c, temporal.ConnectionQueue, worker.Options{})
	connW.RegisterWorkflow(workflow.ConnectionWorkflow)
	// MaintenanceWorkflow is the singleton erasure + GC driver (ADR-0039/0037). The worker
	// only HOSTS it; it is started once out-of-band with the fixed id
	// workflow.MaintenanceWorkflowID (e.g. a `just maintenance-start` admin call), so a
	// missing start never blocks the build or the per-connection sync path.
	connW.RegisterWorkflow(workflow.MaintenanceWorkflow)
	// All activities register on the ConnectionQueue worker (control plane + erasure). The
	// per-connector workers below re-register the same activity struct so SyncUnit is
	// servable on each sync-<connector> queue; Temporal dispatches each activity to the
	// queue the workflow targeted, so the same registration on both queues is correct.
	connW.RegisterActivity(acts)
	workers = append(workers, connW)

	for _, conn := range reg.All() {
		q := temporal.SyncTaskQueue(conn.ID())
		sw := worker.New(c, q, worker.Options{})
		sw.RegisterActivity(acts) // SyncUnit (and siblings) — only SyncUnit is routed here
		workers = append(workers, sw)
		logger.Info("worker_sync_queue_registered", "connector", conn.ID(), "task_queue", q)
	}

	// Liveness HTTP server: /health is process-alive only (it must NOT depend on the
	// DB — a worker with a flaky DB is still a live process the orchestrator should
	// keep, and the activities' own retries handle DB blips). /metrics serves the
	// Prometheus registry.
	stopHTTP := startHealthServer(logger)
	defer stopHTTP()

	logger.Info("worker_listening",
		"host_port", temporal.HostPort(),
		"namespace", temporal.Namespace(),
		"control_queue", temporal.ConnectionQueue,
		"sync_queues", len(workers)-1,
	)

	// Start every worker in this process, then block until SIGINT/SIGTERM (InterruptCh).
	// A single InterruptCh stops them all; on any worker's start error we stop the rest.
	interrupt := worker.InterruptCh()
	for _, w := range workers {
		if err := w.Start(); err != nil {
			for _, started := range workers {
				started.Stop()
			}
			return fmt.Errorf("start worker: %w", err)
		}
	}
	<-interrupt
	for _, w := range workers {
		w.Stop()
	}
	return nil
}

// startHealthServer runs a tiny liveness + metrics server and returns a shutdown
// func. Liveness is process-alive only, by design (see run).
func startHealthServer(logger *slog.Logger) func() {
	port := os.Getenv("HIVEBOOK_WORKER_PORT")
	if port == "" {
		port = "8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"up"}`))
	})
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		logger.Info("worker_health_listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("worker_health_failed", "error", err.Error())
		}
	}()
	return func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}
}
