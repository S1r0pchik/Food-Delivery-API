package domain

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const (
	OrderStatusCreated        OrderStatus = "CREATED"
	OrderStatusConfirmed      OrderStatus = "CONFIRMED"
	OrderStatusCooking        OrderStatus = "COOKING"
	OrderStatusReadyForPickup OrderStatus = "READY_FOR_PICKUP"
	OrderStatusInDelivery     OrderStatus = "IN_DELIVERY"
	OrderStatusDelivered      OrderStatus = "DELIVERED"
	OrderStatusCancelled      OrderStatus = "CANCELLED"
)

type CancellationReason string

const (
	CancellationReasonRejectedByRestaurant CancellationReason = "REJECTED_BY_RESTAURANT"
	CancellationReasonTimeout              CancellationReason = "TIMEOUT_NO_RESTAURANT_REPLY"
	CancellationReasonCancelledByUser      CancellationReason = "CANCELLED_BY_USER"
)

type Order struct {
	ID                          uuid.UUID
	UserID                      uuid.UUID
	RestaurantID                uuid.UUID
	Status                      OrderStatus
	CancellationReason          *CancellationReason
	DeliveryAddress             string
	ContactPhone                string
	Comment                     *string
	TotalAmount                 float64
	EstimatedCookingTimeMinutes *int
	ConfirmationDeadlineAt      time.Time
	Items                       []OrderItemSnapshot
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

type OrderItemSnapshot struct {
	ID           uuid.UUID
	OrderID      uuid.UUID
	MenuItemID   *uuid.UUID
	NameAtOrder  string
	PriceAtOrder float64
	Quantity     int
	TotalPrice   float64
}

func (o *Order) CanTransitionTo(next OrderStatus) bool {
	switch o.Status {
	case OrderStatusCreated:
		return next == OrderStatusConfirmed || next == OrderStatusCancelled
	case OrderStatusConfirmed:
		return next == OrderStatusCooking
	case OrderStatusCooking:
		return next == OrderStatusReadyForPickup
	case OrderStatusReadyForPickup:
		return next == OrderStatusInDelivery
	case OrderStatusInDelivery:
		return next == OrderStatusDelivered
	case OrderStatusDelivered, OrderStatusCancelled:
		return false
	default:
		return false
	}
}

func (o *Order) CanBeCancelledByUser() bool {
	return o.Status == OrderStatusCreated
}

func (o *Order) Confirm(cookingTimeMinutes int) {
	o.Status = OrderStatusConfirmed
	o.EstimatedCookingTimeMinutes = &cookingTimeMinutes
	o.UpdatedAt = time.Now().UTC()
}

func (o *Order) Cancel(reason CancellationReason) {
	o.Status = OrderStatusCancelled
	o.CancellationReason = &reason
	o.UpdatedAt = time.Now().UTC()
}

func (o *Order) Advance(next OrderStatus) {
	o.Status = next
	o.UpdatedAt = time.Now().UTC()
}
