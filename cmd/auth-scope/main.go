package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tauliang/auth-scope/internal/mission"
	"github.com/tauliang/auth-scope/internal/mission/store"
)

func main() {
	addr := os.Getenv("AUTH_SCOPE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var ms mission.Store
	databaseURL := os.Getenv("DATABASE_URL")
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

	service := mission.NewService(ms, mission.SystemClock{})
	handler := mission.NewHandler(service)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("auth-scope listening", "addr", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if databaseURL != "" {
		publisher := mission.NewOutboxPublisher(ms, 500*time.Millisecond)
		go func() {
			if err := publisher.Start(ctx); err != nil {
				slog.Error("outbox publisher stopped", "error", err)
			}
		}()
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
