package usecase

import (
	"context"

	"food-delivery-api/internal/core/domain"
)

type WebhookSender interface {
	SendOrderCreated(ctx context.Context, restaurant *domain.Restaurant, order *domain.Order) error
}
