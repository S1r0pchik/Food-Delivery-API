package domain

import "errors"

var (
	ErrRestaurantNotFound = errors.New("restaurant not found")
	ErrRestaurantClosed   = errors.New("restaurant is currently closed")

	ErrMenuItemNotFound      = errors.New("menu item not found")
	ErrForeignRestaurantItem = errors.New("menu item does not belong to the restaurant")

	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidStatusOrder = errors.New("invalid order status transition")
	ErrCannotCancelOrder  = errors.New("order cannot be cancelled at this stage")
	ErrEmptyOrderItems    = errors.New("order must contain at least one item")
	ErrInvalidOrder       = errors.New("invalid order")
)
