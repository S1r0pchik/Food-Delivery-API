package usecase_test

import (
	"context"
	"testing"

	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"

	"github.com/google/uuid"
)

func TestPartnerUsecase_AcceptOrder_Success(t *testing.T) {
	restID := uuid.New()
	orderID := uuid.New()

	order := &domain.Order{
		ID:           orderID,
		RestaurantID: restID,
		Status:       domain.OrderStatusCreated,
	}

	orderRepo := &mockOrderRepo{savedOrder: order}
	restRepo := &mockRestaurantRepo{restaurant: &domain.Restaurant{ID: restID}}
	menuRepo := &mockMenuRepo{}

	uc := usecase.NewPartnerUsecase(orderRepo, restRepo, menuRepo)

	updatedOrder, err := uc.AcceptOrder(context.Background(), restID, orderID, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedOrder.Status != domain.OrderStatusConfirmed {
		t.Errorf("expected status %s, got %s", domain.OrderStatusConfirmed, updatedOrder.Status)
	}
	if updatedOrder.EstimatedCookingTimeMinutes == nil || *updatedOrder.EstimatedCookingTimeMinutes != 30 {
		t.Errorf("expected cooking time 30, got %v", updatedOrder.EstimatedCookingTimeMinutes)
	}
}

func TestPartnerUsecase_AcceptOrder_ForeignRestaurant(t *testing.T) {
	restID := uuid.New()
	otherRestID := uuid.New()
	orderID := uuid.New()

	order := &domain.Order{
		ID:           orderID,
		RestaurantID: otherRestID,
		Status:       domain.OrderStatusCreated,
	}

	orderRepo := &mockOrderRepo{savedOrder: order}
	restRepo := &mockRestaurantRepo{restaurant: &domain.Restaurant{ID: restID}}
	menuRepo := &mockMenuRepo{}

	uc := usecase.NewPartnerUsecase(orderRepo, restRepo, menuRepo)

	_, err := uc.AcceptOrder(context.Background(), restID, orderID, 20)
	if err != domain.ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound for foreign restaurant, got %v", err)
	}
}

func TestPartnerUsecase_UpdateKitchenStatus_InvalidTransition(t *testing.T) {
	restID := uuid.New()
	orderID := uuid.New()

	order := &domain.Order{
		ID:           orderID,
		RestaurantID: restID,
		Status:       domain.OrderStatusCreated,
	}

	orderRepo := &mockOrderRepo{savedOrder: order}
	restRepo := &mockRestaurantRepo{restaurant: &domain.Restaurant{ID: restID}}
	menuRepo := &mockMenuRepo{}

	uc := usecase.NewPartnerUsecase(orderRepo, restRepo, menuRepo)

	_, err := uc.UpdateKitchenStatus(context.Background(), restID, orderID, domain.OrderStatusReadyForPickup)
	if err != domain.ErrInvalidStatusOrder {
		t.Errorf("expected ErrInvalidStatusOrder, got %v", err)
	}
}

func TestPartnerUsecase_RejectOrder_Success(t *testing.T) {
	restID := uuid.New()
	orderID := uuid.New()

	order := &domain.Order{
		ID:           orderID,
		RestaurantID: restID,
		Status:       domain.OrderStatusCreated,
	}

	orderRepo := &mockOrderRepo{savedOrder: order}
	restRepo := &mockRestaurantRepo{restaurant: &domain.Restaurant{ID: restID}}
	menuRepo := &mockMenuRepo{}

	uc := usecase.NewPartnerUsecase(orderRepo, restRepo, menuRepo)

	rejOrder, err := uc.RejectOrder(context.Background(), restID, orderID, domain.CancellationReasonRejectedByRestaurant)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rejOrder.Status != domain.OrderStatusCancelled {
		t.Errorf("expected status %s, got %s", domain.OrderStatusCancelled, rejOrder.Status)
	}
}

func TestPartnerUsecase_UpdateItemAvailability(t *testing.T) {
	restID := uuid.New()
	itemID := uuid.New()

	orderRepo := &mockOrderRepo{}
	restRepo := &mockRestaurantRepo{restaurant: &domain.Restaurant{ID: restID}}
	menuRepo := &mockMenuRepo{
		items: []domain.MenuItem{
			{ID: itemID, RestaurantID: restID, IsAvailable: true},
		},
	}

	uc := usecase.NewPartnerUsecase(orderRepo, restRepo, menuRepo)

	item, err := uc.UpdateItemAvailability(context.Background(), restID, itemID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.IsAvailable {
		t.Errorf("expected item to be unavailable")
	}
}
