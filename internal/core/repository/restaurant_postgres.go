package repository

import (
	"context"
	"errors"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RestaurantPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewRestaurantPostgresRepo(pool *pgxpool.Pool) *RestaurantPostgresRepo {
	return &RestaurantPostgresRepo{pool: pool}
}

const restaurantColumns = `
	SELECT id, name, address, is_open, cuisine_type, webhook_url, api_key, 
	       webhook_secret, estimated_cooking_time_minutes, created_at, updated_at
	FROM restaurants`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRestaurant(s rowScanner) (domain.Restaurant, error) {
	var rest domain.Restaurant
	err := s.Scan(
		&rest.ID, &rest.Name, &rest.Address, &rest.IsOpen, &rest.CuisineType,
		&rest.WebhookURL, &rest.APIKey, &rest.WebhookSecret,
		&rest.EstimatedCookingTimeMinutes, &rest.CreatedAt, &rest.UpdatedAt,
	)
	return rest, err
}

func (r *RestaurantPostgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	row := r.pool.QueryRow(ctx, restaurantColumns+` WHERE id = $1`, id)
	rest, err := scanRestaurant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRestaurantNotFound
		}
		return nil, err
	}
	return &rest, nil
}

func (r *RestaurantPostgresRepo) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Restaurant, error) {
	row := r.pool.QueryRow(ctx, restaurantColumns+` WHERE api_key = $1`, apiKey)
	rest, err := scanRestaurant(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRestaurantNotFound
		}
		return nil, err
	}
	return &rest, nil
}

func (r *RestaurantPostgresRepo) List(ctx context.Context, openOnly bool, limit, offset int) ([]domain.Restaurant, error) {
	query := restaurantColumns + `
	WHERE ($1 = false OR is_open = true)
	ORDER BY name ASC
	LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, openOnly, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var restaurants []domain.Restaurant
	for rows.Next() {
		rest, err := scanRestaurant(rows)
		if err != nil {
			return nil, err
		}
		restaurants = append(restaurants, rest)
	}

	return restaurants, rows.Err()
}
