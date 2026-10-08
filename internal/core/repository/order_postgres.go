package repository

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrderPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewOrderPostgresRepo(pool *pgxpool.Pool) *OrderPostgresRepo {
	return &OrderPostgresRepo{pool: pool}
}

func (r *OrderPostgresRepo) Create(ctx context.Context, order *domain.Order) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("Failed to roll back order transaction", "order_id", order.ID, "error", err)
		}
	}()

	orderQuery := `
		INSERT INTO orders (
			id, user_id, restaurant_id, status, cancellation_reason,
			delivery_address, contact_phone, comment, total_amount,
			estimated_cooking_time_minutes, confirmation_deadline_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err = tx.Exec(ctx, orderQuery,
		order.ID, order.UserID, order.RestaurantID, order.Status, order.CancellationReason,
		order.DeliveryAddress, order.ContactPhone, order.Comment, order.TotalAmount,
		order.EstimatedCookingTimeMinutes, order.ConfirmationDeadlineAt, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return err
	}

	itemQuery := `
		INSERT INTO order_items (
			id, order_id, menu_item_id, name_at_order, price_at_order, quantity, total_price, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	for _, it := range order.Items {
		_, err := tx.Exec(ctx, itemQuery,
			it.ID, it.OrderID, it.MenuItemID, it.NameAtOrder, it.PriceAtOrder, it.Quantity, it.TotalPrice, order.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *OrderPostgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	orderQuery := `
		SELECT id, user_id, restaurant_id, status, cancellation_reason,
		       delivery_address, contact_phone, comment, total_amount,
		       estimated_cooking_time_minutes, confirmation_deadline_at, created_at, updated_at
		FROM orders
		WHERE id = $1`

	var order domain.Order
	err := r.pool.QueryRow(ctx, orderQuery, id).Scan(
		&order.ID, &order.UserID, &order.RestaurantID, &order.Status, &order.CancellationReason,
		&order.DeliveryAddress, &order.ContactPhone, &order.Comment, &order.TotalAmount,
		&order.EstimatedCookingTimeMinutes, &order.ConfirmationDeadlineAt, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}

	itemsQuery := `
		SELECT id, order_id, menu_item_id, name_at_order, price_at_order, quantity, total_price
		FROM order_items
		WHERE order_id = $1`

	rows, err := r.pool.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.OrderItemSnapshot
		if err := rows.Scan(
			&item.ID, &item.OrderID, &item.MenuItemID, &item.NameAtOrder,
			&item.PriceAtOrder, &item.Quantity, &item.TotalPrice,
		); err != nil {
			return nil, err
		}
		order.Items = append(order.Items, item)
	}

	return &order, rows.Err()
}

func (r *OrderPostgresRepo) UpdateStatus(
	ctx context.Context,
	orderID uuid.UUID,
	status domain.OrderStatus,
	reason *domain.CancellationReason,
	cookingTime *int,
) error {
	query := `
		UPDATE orders
		SET status = $1,
		    cancellation_reason = COALESCE($2, cancellation_reason),
		    estimated_cooking_time_minutes = COALESCE($3, estimated_cooking_time_minutes),
		    updated_at = clock_timestamp()
		WHERE id = $4`

	cmd, err := r.pool.Exec(ctx, query, status, reason, cookingTime, orderID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrOrderNotFound
	}

	return nil
}

func (r *OrderPostgresRepo) GetExpiredCreatedOrders(ctx context.Context, deadline time.Time, limit int) ([]domain.Order, error) {
	query := `
		SELECT id, user_id, restaurant_id, status, cancellation_reason,
		       delivery_address, contact_phone, comment, total_amount,
		       estimated_cooking_time_minutes, confirmation_deadline_at, created_at, updated_at
		FROM orders
		WHERE status = 'CREATED' AND confirmation_deadline_at <= $1
		ORDER BY confirmation_deadline_at ASC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, deadline, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var order domain.Order
		if err := rows.Scan(
			&order.ID, &order.UserID, &order.RestaurantID, &order.Status, &order.CancellationReason,
			&order.DeliveryAddress, &order.ContactPhone, &order.Comment, &order.TotalAmount,
			&order.EstimatedCookingTimeMinutes, &order.ConfirmationDeadlineAt, &order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return orders, rows.Err()
}
