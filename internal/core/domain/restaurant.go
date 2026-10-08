package domain

import (
	"time"

	"github.com/google/uuid"
)

type Restaurant struct {
	ID                          uuid.UUID
	Name                        string
	Address                     string
	IsOpen                      bool
	CuisineType                 string
	WebhookURL                  string
	APIKey                      string
	WebhookSecret               string
	EstimatedCookingTimeMinutes int
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}
