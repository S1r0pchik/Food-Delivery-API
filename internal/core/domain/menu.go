package domain

import (
	"time"

	"github.com/google/uuid"
)

type MenuCategory struct {
	ID           uuid.UUID
	RestaurantID uuid.UUID
	Name         string
	SortOrder    int
	Items        []MenuItem
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MenuItem struct {
	ID           uuid.UUID
	RestaurantID uuid.UUID
	CategoryID   uuid.UUID
	Name         string
	Description  string
	Price        float64
	IsAvailable  bool
	PhotoURL     *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
