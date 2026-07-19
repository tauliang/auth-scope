package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tauliang/auth-scope/internal/mission"
	"github.com/tauliang/auth-scope/internal/mission/store"
)

func main() {
	production := mission.ProductionModeFromEnv()
	addr := os.Getenv("AUTH_SCOPE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var ms mission.Store
	databaseURL := os.Getenv("DATABASE_URL")
	if production && databaseURL == "" {
		slog.Error("DATABASE_URL is required when AUTH_SCOPE_MODE=production")
		os.Exit(1)
	}
	if databaseURL != "" {
		db, err := store.NewPostgresStoreFromEnv()
		if err != nil {
			slog.Error("failed to create postgres store", "error", err)
			os.Exit(1)
		}
		ms = db
		defer db.Close()
		slog.Info("using postgres store")
	} else {
		ms = mission.NewMemoryStore()
		slog.Info("using in-memory store (no DATABASE_URL set)")
	}

	artifactKey, err := mission.ArtifactKeyFromEnv(production)
	if err != nil {
		slog.Error("invalid artifact signing configuration", "error", err)
		os.Exit(1)
	}
	adminAuthenticator, err := mission.AdminAuthenticatorFromEnvStrict(production)
	if err != nil {
		slog.Error("invalid admin authentication configuration", "error", err)
		os.Exit(1)
	}

	service := mission.NewServiceWithArtifactKey(ms, mission.SystemClock{}, artifactKey)
	handler := mission.NewHandlerWithOptions(service, adminAuthenticator, mission.HandlerOptions{RequireAgentSignatures: true})

	server := &http.Server{
		Addr:              addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("auth-scope listening", "addr", addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	outboxCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	if databaseURL != "" {
		publisher := mission.NewOutboxPublisher(ms, 500*time.Millisecond)
		go func() {
			if err := publisher.Start(outboxCtx); err != nil {
				slog.Error("outbox publisher stopped", "error", err)
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
		slog.Info("auth-scope stopped")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}
}
