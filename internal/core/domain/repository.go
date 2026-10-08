package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RestaurantRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Restaurant, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*Restaurant, error)
	List(ctx context.Context, openOnly bool, limit, offset int) ([]Restaurant, error)
}

type MenuRepository interface {
	GetMenuByRestaurantID(ctx context.Context, restaurantID uuid.UUID) ([]MenuCategory, error)
	GetItemsByIDs(ctx context.Context, itemIDs []uuid.UUID) ([]MenuItem, error)
	UpdateItemAvailability(ctx context.Context, itemID uuid.UUID, isAvailable bool) (*MenuItem, error)
}

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
	UpdateStatus(ctx context.Context, orderID uuid.UUID, status OrderStatus, reason *CancellationReason, cookingTime *int) error
	GetExpiredCreatedOrders(ctx context.Context, deadline time.Time, limit int) ([]Order, error)
}
