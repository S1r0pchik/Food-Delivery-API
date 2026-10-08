package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"food-delivery-api/internal/restaurant/client"
	"food-delivery-api/internal/restaurant/config"
	"food-delivery-api/internal/restaurant/handler"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	slog.Info("Starting Restaurant Mock Service...", "port", cfg.HTTPPort)

	coreClient := client.NewCoreClient(cfg.CoreBaseURL, cfg.APIKey)
	webhookHandler := handler.NewWebhookHandler(cfg, coreClient)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Post("/webhooks/orders/incoming", webhookHandler.HandleIncomingOrder)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			slog.Error("Failed to write health response", "error", err)
		}
	})

	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("Restaurant Mock HTTP server listening", "port", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Restaurant Mock server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down Restaurant Mock gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Restaurant Mock shutdown failed", "error", err)
	}
	slog.Info("Restaurant Mock stopped")
}
