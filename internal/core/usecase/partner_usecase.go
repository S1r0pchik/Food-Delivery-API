package usecase

import (
	"context"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
)

type PartnerUsecase struct {
	orderRepo      domain.OrderRepository
	restaurantRepo domain.RestaurantRepository
	menuRepo       domain.MenuRepository
}

func NewPartnerUsecase(
	orderRepo domain.OrderRepository,
	restaurantRepo domain.RestaurantRepository,
	menuRepo domain.MenuRepository,
) *PartnerUsecase {
	return &PartnerUsecase{
		orderRepo:      orderRepo,
		restaurantRepo: restaurantRepo,
		menuRepo:       menuRepo,
	}
}

func (u *PartnerUsecase) AcceptOrder(ctx context.Context, restaurantID, orderID uuid.UUID, cookingTimeMinutes int) (*domain.Order, error) {
	order, err := u.getRestaurantOrder(ctx, restaurantID, orderID)
	if err != nil {
		return nil, err
	}

	if !order.CanTransitionTo(domain.OrderStatusConfirmed) {
		return nil, domain.ErrInvalidStatusOrder
	}

	if err := u.orderRepo.UpdateStatus(ctx, orderID, domain.OrderStatusConfirmed, nil, &cookingTimeMinutes); err != nil {
		return nil, err
	}

	order.Confirm(cookingTimeMinutes)
	return order, nil
}

func (u *PartnerUsecase) RejectOrder(ctx context.Context, restaurantID, orderID uuid.UUID, reason domain.CancellationReason) (*domain.Order, error) {
	order, err := u.getRestaurantOrder(ctx, restaurantID, orderID)
	if err != nil {
		return nil, err
	}

	if !order.CanTransitionTo(domain.OrderStatusCancelled) {
		return nil, domain.ErrInvalidStatusOrder
	}

	if err := u.orderRepo.UpdateStatus(ctx, orderID, domain.OrderStatusCancelled, &reason, nil); err != nil {
		return nil, err
	}

	order.Cancel(reason)
	return order, nil
}

func (u *PartnerUsecase) UpdateKitchenStatus(ctx context.Context, restaurantID, orderID uuid.UUID, nextStatus domain.OrderStatus) (*domain.Order, error) {
	order, err := u.getRestaurantOrder(ctx, restaurantID, orderID)
	if err != nil {
		return nil, err
	}

	if nextStatus != domain.OrderStatusCooking && nextStatus != domain.OrderStatusReadyForPickup {
		return nil, domain.ErrInvalidStatusOrder
	}

	if !order.CanTransitionTo(nextStatus) {
		return nil, domain.ErrInvalidStatusOrder
	}

	if err := u.orderRepo.UpdateStatus(ctx, orderID, nextStatus, nil, nil); err != nil {
		return nil, err
	}

	order.Advance(nextStatus)
	return order, nil
}

func (u *PartnerUsecase) UpdateItemAvailability(ctx context.Context, restaurantID, itemID uuid.UUID, isAvailable bool) (*domain.MenuItem, error) {
	items, err := u.menuRepo.GetItemsByIDs(ctx, []uuid.UUID{itemID})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 || items[0].RestaurantID != restaurantID {
		return nil, domain.ErrMenuItemNotFound
	}

	return u.menuRepo.UpdateItemAvailability(ctx, itemID, isAvailable)
}

func (u *PartnerUsecase) getRestaurantOrder(ctx context.Context, restaurantID, orderID uuid.UUID) (*domain.Order, error) {
	order, err := u.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.RestaurantID != restaurantID {
		return nil, domain.ErrOrderNotFound
	}
	return order, nil
}
