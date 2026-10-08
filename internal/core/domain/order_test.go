package domain_test

import (
	"testing"

	"food-delivery-api/internal/core/domain"
)

func TestOrder_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		from     domain.OrderStatus
		to       domain.OrderStatus
		expected bool
	}{
		{"CREATED -> CONFIRMED", domain.OrderStatusCreated, domain.OrderStatusConfirmed, true},
		{"CREATED -> CANCELLED", domain.OrderStatusCreated, domain.OrderStatusCancelled, true},
		{"CREATED -> COOKING", domain.OrderStatusCreated, domain.OrderStatusCooking, false},
		{"CONFIRMED -> COOKING", domain.OrderStatusConfirmed, domain.OrderStatusCooking, true},
		{"CONFIRMED -> CANCELLED", domain.OrderStatusConfirmed, domain.OrderStatusCancelled, false},
		{"COOKING -> READY_FOR_PICKUP", domain.OrderStatusCooking, domain.OrderStatusReadyForPickup, true},
		{"COOKING -> CANCELLED", domain.OrderStatusCooking, domain.OrderStatusCancelled, false},
		{"READY_FOR_PICKUP -> IN_DELIVERY", domain.OrderStatusReadyForPickup, domain.OrderStatusInDelivery, true},
		{"IN_DELIVERY -> DELIVERED", domain.OrderStatusInDelivery, domain.OrderStatusDelivered, true},
		{"DELIVERED -> CANCELLED", domain.OrderStatusDelivered, domain.OrderStatusCancelled, false},
		{"CANCELLED -> CONFIRMED", domain.OrderStatusCancelled, domain.OrderStatusConfirmed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := domain.Order{Status: tt.from}
			if got := order.CanTransitionTo(tt.to); got != tt.expected {
				t.Errorf("CanTransitionTo(%s -> %s) = %v; want %v", tt.from, tt.to, got, tt.expected)
			}
		})
	}
}

func TestOrder_CanBeCancelledByUser(t *testing.T) {
	cancellable := []domain.OrderStatus{
		domain.OrderStatusCreated,
	}
	for _, st := range cancellable {
		order := domain.Order{Status: st}
		if !order.CanBeCancelledByUser() {
			t.Errorf("status %s should be cancellable by user", st)
		}
	}

	nonCancellable := []domain.OrderStatus{
		domain.OrderStatusConfirmed,
		domain.OrderStatusCooking,
		domain.OrderStatusReadyForPickup,
		domain.OrderStatusInDelivery,
		domain.OrderStatusDelivered,
		domain.OrderStatusCancelled,
	}
	for _, st := range nonCancellable {
		order := domain.Order{Status: st}
		if order.CanBeCancelledByUser() {
			t.Errorf("status %s should NOT be cancellable by user", st)
		}
	}
}
