package repository

import (
	"context"
	"errors"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MenuPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewMenuPostgresRepo(pool *pgxpool.Pool) *MenuPostgresRepo {
	return &MenuPostgresRepo{pool: pool}
}

func (r *MenuPostgresRepo) GetMenuByRestaurantID(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuCategory, error) {
	catQuery := `
		SELECT id, restaurant_id, name, sort_order, created_at, updated_at
		FROM menu_categories
		WHERE restaurant_id = $1
		ORDER BY sort_order ASC`

	catRows, err := r.pool.Query(ctx, catQuery, restaurantID)
	if err != nil {
		return nil, err
	}
	defer catRows.Close()

	var categories []domain.MenuCategory
	categoryIndexMap := make(map[uuid.UUID]int)

	for catRows.Next() {
		var cat domain.MenuCategory
		if err := catRows.Scan(
			&cat.ID, &cat.RestaurantID, &cat.Name, &cat.SortOrder,
			&cat.CreatedAt, &cat.UpdatedAt,
		); err != nil {
			return nil, err
		}
		categoryIndexMap[cat.ID] = len(categories)
		categories = append(categories, cat)
	}
	if err := catRows.Err(); err != nil {
		return nil, err
	}

	itemsQuery := `
		SELECT id, restaurant_id, category_id, name, description, price, is_available, photo_url, created_at, updated_at
		FROM menu_items
		WHERE restaurant_id = $1
		ORDER BY name ASC`

	itemRows, err := r.pool.Query(ctx, itemsQuery, restaurantID)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item domain.MenuItem
		if err := itemRows.Scan(
			&item.ID, &item.RestaurantID, &item.CategoryID, &item.Name,
			&item.Description, &item.Price, &item.IsAvailable, &item.PhotoURL,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if idx, exists := categoryIndexMap[item.CategoryID]; exists {
			categories[idx].Items = append(categories[idx].Items, item)
		}
	}

	return categories, itemRows.Err()
}

func (r *MenuPostgresRepo) GetItemsByIDs(ctx context.Context, itemIDs []uuid.UUID) ([]domain.MenuItem, error) {
	query := `
		SELECT id, restaurant_id, category_id, name, description, price, is_available, photo_url, created_at, updated_at
		FROM menu_items
		WHERE id = ANY($1)`

	rows, err := r.pool.Query(ctx, query, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.MenuItem
	for rows.Next() {
		var item domain.MenuItem
		if err := rows.Scan(
			&item.ID, &item.RestaurantID, &item.CategoryID, &item.Name,
			&item.Description, &item.Price, &item.IsAvailable, &item.PhotoURL,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *MenuPostgresRepo) UpdateItemAvailability(ctx context.Context, itemID uuid.UUID, isAvailable bool) (*domain.MenuItem, error) {
	query := `
		UPDATE menu_items
		SET is_available = $1, updated_at = clock_timestamp()
		WHERE id = $2
		RETURNING id, restaurant_id, category_id, name, description, price, is_available, photo_url, created_at, updated_at`

	var item domain.MenuItem
	err := r.pool.QueryRow(ctx, query, isAvailable, itemID).Scan(
		&item.ID, &item.RestaurantID, &item.CategoryID, &item.Name,
		&item.Description, &item.Price, &item.IsAvailable, &item.PhotoURL,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMenuItemNotFound
		}
		return nil, err
	}

	return &item, nil
}
