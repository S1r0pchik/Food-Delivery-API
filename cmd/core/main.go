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

	"food-delivery-api/internal/core/client"
	"food-delivery-api/internal/core/config"
	deliveryhttp "food-delivery-api/internal/core/delivery/http"
	v1 "food-delivery-api/internal/core/delivery/http/v1"
	"food-delivery-api/internal/core/repository"
	"food-delivery-api/internal/core/usecase"
	"food-delivery-api/internal/core/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	slog.Info("Starting Food Delivery API Core Service...")

	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()

	db, err := repository.NewPostgresDB(initCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("Connected to PostgreSQL successfully")

	restaurantRepo := repository.NewRestaurantPostgresRepo(db.Pool)
	menuRepo := repository.NewMenuPostgresRepo(db.Pool)
	orderRepo := repository.NewOrderPostgresRepo(db.Pool)
	webhookSender := client.NewRestaurantWebhookClient()

	orderUsecase := usecase.NewOrderUsecase(orderRepo, menuRepo, restaurantRepo, webhookSender)
	partnerUsecase := usecase.NewPartnerUsecase(orderRepo, restaurantRepo, menuRepo)
	catalogUsecase := usecase.NewCatalogUsecase(restaurantRepo, menuRepo)

	handlers := deliveryhttp.Handlers{
		ClientHandler:   v1.NewClientHandler(catalogUsecase, orderUsecase),
		PartnerHandler:  v1.NewPartnerHandler(partnerUsecase),
		PlatformHandler: v1.NewPlatformHandler(orderUsecase),
		RestaurantRepo:  restaurantRepo,
	}

	router := deliveryhttp.NewRouter(handlers)
	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	timeoutWorker := worker.NewTimeoutWorker(orderRepo, cfg.OrderTimeoutCheckInterval)
	go timeoutWorker.Start(ctx)

	go func() {
		slog.Info("Core HTTP server listening", "port", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("Shutdown signal received, draining connections...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server forced to shutdown", "error", err)
	}

	slog.Info("Food Delivery API Core Service stopped gracefully")
}
