// Command integrations runs the integrations Connect API (the platform API
// proxies to it). The sync orchestration is being rewritten onto Temporal
// (design-doc 0012); the old River enqueuer/worker wiring has been removed.
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"integrations/internal/connectors"
	"integrations/internal/database"
	"integrations/internal/events"
	"integrations/internal/metrics"
	"integrations/internal/oauth"
	"integrations/internal/server"
	"integrations/internal/storage"
	"integrations/internal/temporal"
)

// metricsInterval is how often the backlog exporter refreshes the aggregate SLIs and
// the Temporal task-queue backlog (~15s — fresh enough for KEDA, cheap enough to ignore).
const metricsInterval = 15 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("integrations_init_failed", "error", err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
	signer, err := oauth.NewSigner([]byte(os.Getenv("HIVEBOOK_INTEGRATIONS_STATE_KEY")))
	if err != nil {
		return fmt.Errorf("state signer: %w", err)
	}

	reg := connectors.NewRegistry(connectors.NewGoogleDocs(), connectors.NewGitHub(), connectors.NewGitHubPAT())
	creds := connectors.CredsFromEnv(reg)

	// One Postgres LISTEN connection fans connection-change events out to the
	// WatchConnections streams (cheap, event-driven — no polling).
	broker := events.NewBroker()
	go events.Listen(ctx, database.ConfigFromEnv().DSN(), broker)

	st, err := storage.New(ctx, storage.ConfigFromEnv())
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}

	// OTel tracing, gated on HIVEBOOK_OTLP_ENDPOINT (no-op unset). Install before Dial so
	// the client's tracing interceptor uses the real provider when an endpoint is set.
	shutdownTracing, err := temporal.InitTracing(ctx, "integrations")
	if err != nil {
		return fmt.Errorf("tracing: %w", err)
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutCtx)
	}()

	// The integrations service is a Temporal CLIENT: RPCs translate into signals to the
	// per-connection ConnectionWorkflow (design-doc 0012). The worker (cmd/worker) hosts
	// the workflows + activities; this process only signals them.
	tc, err := temporal.Dial(slog.Default())
	if err != nil {
		return fmt.Errorf("temporal: %w", err)
	}
	defer tc.Close()

	// The always-on backlog exporter (design-doc 0012, "Observability"): every cycle it
	// refreshes the aggregate outcome SLIs from Postgres and the Temporal task-queue
	// backlog (the KEDA scale signal) onto the default registry the server's /metrics
	// already serves. Runs here, in the always-on server — not the scale-to-zero worker.
	connectorIDs := make(metrics.Connectors, 0, len(reg.All()))
	for _, c := range reg.All() {
		connectorIDs = append(connectorIDs, c.ID())
	}
	go metrics.RunExporter(ctx, store.SQLDB(), tc, connectorIDs, metricsInterval)

	srv := server.New(store, reg, cipher, signer, creds, broker, st, tc, os.Getenv("HIVEBOOK_INTEGRATIONS_CALLBACK_BASE"))

	port := os.Getenv("HIVEBOOK_INTEGRATIONS_PORT")
	if port == "" {
		port = "8080"
	}
	httpSrv := &http.Server{
		Addr:    ":" + port,
		Handler: srv.Handler(),
		// No WriteTimeout: WatchConnections is a long-lived server stream and a
		// finite write deadline would cut it. ReadHeaderTimeout keeps slow-loris
		// protection; IdleTimeout reaps dead keep-alives.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("integrations_listening", "addr", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("integrations_shutdown")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutCtx)
	}
}
