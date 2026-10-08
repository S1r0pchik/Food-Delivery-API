package worker

import (
	"context"
	"log/slog"
	"time"

	"food-delivery-api/internal/core/domain"
)

type TimeoutWorker struct {
	orderRepo     domain.OrderRepository
	checkInterval time.Duration
	batchSize     int
}

func NewTimeoutWorker(orderRepo domain.OrderRepository, checkInterval time.Duration) *TimeoutWorker {
	return &TimeoutWorker{
		orderRepo:     orderRepo,
		checkInterval: checkInterval,
		batchSize:     50,
	}
}

func (w *TimeoutWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	slog.Info("Timeout worker started", "interval", w.checkInterval.String())

	for {
		select {
		case <-ctx.Done():
			slog.Info("Timeout worker shutting down gracefully")
			return
		case <-ticker.C:
			w.processExpiredOrders(ctx)
		}
	}
}

func (w *TimeoutWorker) processExpiredOrders(ctx context.Context) {
	now := time.Now().UTC()
	expiredOrders, err := w.orderRepo.GetExpiredCreatedOrders(ctx, now, w.batchSize)
	if err != nil {
		slog.Error("Failed to query expired orders", "error", err)
		return
	}

	if len(expiredOrders) == 0 {
		return
	}

	reason := domain.CancellationReasonTimeout
	for _, order := range expiredOrders {
		err := w.orderRepo.UpdateStatus(ctx, order.ID, domain.OrderStatusCancelled, &reason, nil)
		if err != nil {
			slog.Error("Failed to cancel expired order", "order_id", order.ID, "error", err)
			continue
		}
		slog.Warn("Order automatically cancelled due to restaurant timeout",
			"order_id", order.ID,
			"restaurant_id", order.RestaurantID,
			"deadline", order.ConfirmationDeadlineAt,
		)
	}
}
