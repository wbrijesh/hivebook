package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"api/internal/server"
)

func gracefulShutdown(apiServer *http.Server, done chan bool) {
	// Listen for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("shutdown_initiated")
	stop() // Allow a second Ctrl+C to force shutdown.

	// Give in-flight requests 5 seconds to finish.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		slog.Error("shutdown_error", "error", err.Error())
	}

	slog.Info("server_exiting")
	done <- true
}

func main() {
	// Structured JSON logs to stdout, collected by Vector -> VictoriaLogs.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	apiServer, err := server.NewServer()
	if err != nil {
		slog.Error("server_init_failed", "error", err.Error())
		os.Exit(1)
	}

	done := make(chan bool, 1)
	go gracefulShutdown(apiServer, done)

	slog.Info("server_listening", "addr", apiServer.Addr)
	if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server_error", "error", err.Error())
		os.Exit(1)
	}

	<-done
	slog.Info("shutdown_complete")
}
