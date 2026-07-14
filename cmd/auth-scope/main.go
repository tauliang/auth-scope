package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/tauliang/auth-scope/internal/mission"
)

func main() {
	addr := os.Getenv("AUTH_SCOPE_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	store := mission.NewMemoryStore()
	service := mission.NewService(store, mission.SystemClock{})
	handler := mission.NewHandler(service)

	server := &http.Server{
		Addr:              addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("auth-scope listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
