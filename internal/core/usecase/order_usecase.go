package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
)

type CreateOrderItemInput struct {
	MenuItemID    uuid.UUID
	Quantity      int
	ExpectedPrice float64
}

type CreateOrderInput struct {
	UserID          uuid.UUID
	RestaurantID    uuid.UUID
	DeliveryAddress string
	ContactPhone    string
	Comment         *string
	Items           []CreateOrderItemInput
}

type OrderConflictError struct {
	UnavailableItemIDs []uuid.UUID
	PriceDiscrepancies []PriceDiscrepancy
}

type PriceDiscrepancy struct {
	MenuItemID    uuid.UUID
	ExpectedPrice float64
	ActualPrice   float64
}

func (e *OrderConflictError) Error() string {
	return fmt.Sprintf("order conflict: %d unavailable items, %d price discrepancies",
		len(e.UnavailableItemIDs), len(e.PriceDiscrepancies))
}

type OrderUsecase struct {
	orderRepo      domain.OrderRepository
	menuRepo       domain.MenuRepository
	restaurantRepo domain.RestaurantRepository
	webhookSender  WebhookSender
}

func NewOrderUsecase(
	orderRepo domain.OrderRepository,
	menuRepo domain.MenuRepository,
	restaurantRepo domain.RestaurantRepository,
	webhookSender WebhookSender,
) *OrderUsecase {
	return &OrderUsecase{
		orderRepo:      orderRepo,
		menuRepo:       menuRepo,
		restaurantRepo: restaurantRepo,
		webhookSender:  webhookSender,
	}
}

func (u *OrderUsecase) CreateOrder(ctx context.Context, in CreateOrderInput) (*domain.Order, error) {
	if err := validateCreateOrder(in); err != nil {
		return nil, err
	}

	rest, err := u.restaurantRepo.GetByID(ctx, in.RestaurantID)
	if err != nil {
		return nil, err
	}
	if !rest.IsOpen {
		return nil, domain.ErrRestaurantClosed
	}

	menuItems, err := u.fetchAndValidateItems(ctx, in)
	if err != nil {
		return nil, err
	}

	order := u.assembleOrder(in, menuItems)
	if err := u.orderRepo.Create(ctx, order); err != nil {
		return nil, err
	}

	go u.notifyRestaurant(rest, order)

	return order, nil
}

func validateCreateOrder(in CreateOrderInput) error {
	if len(in.Items) == 0 {
		return domain.ErrEmptyOrderItems
	}

	if strings.TrimSpace(in.DeliveryAddress) == "" {
		return fmt.Errorf("%w: delivery_address is required", domain.ErrInvalidOrder)
	}

	if strings.TrimSpace(in.ContactPhone) == "" {
		return fmt.Errorf("%w: contact_phone is required", domain.ErrInvalidOrder)
	}

	seen := make(map[uuid.UUID]struct{}, len(in.Items))
	for _, item := range in.Items {
		if item.Quantity <= 0 {
			return fmt.Errorf("%w: quantity must be positive for item %s", domain.ErrInvalidOrder, item.MenuItemID)
		}
		if _, exists := seen[item.MenuItemID]; exists {
			return fmt.Errorf("%w: duplicate item %s", domain.ErrInvalidOrder, item.MenuItemID)
		}
		seen[item.MenuItemID] = struct{}{}
	}

	return nil
}

func (u *OrderUsecase) GetOrderByID(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	return u.orderRepo.GetByID(ctx, orderID)
}

func (u *OrderUsecase) CancelOrderByUser(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error) {
	order, err := u.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	if order.UserID != userID {
		return nil, domain.ErrOrderNotFound
	}
	if !order.CanBeCancelledByUser() {
		return nil, domain.ErrCannotCancelOrder
	}

	reason := domain.CancellationReasonCancelledByUser
	if err := u.orderRepo.UpdateStatus(ctx, orderID, domain.OrderStatusCancelled, &reason, nil); err != nil {
		return nil, err
	}

	order.Cancel(reason)
	return order, nil
}

func (u *OrderUsecase) AdvanceDeliveryStep(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	order, err := u.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	var next domain.OrderStatus
	switch order.Status {
	case domain.OrderStatusReadyForPickup:
		next = domain.OrderStatusInDelivery
	case domain.OrderStatusInDelivery:
		next = domain.OrderStatusDelivered
	default:
		return nil, domain.ErrInvalidStatusOrder
	}

	if !order.CanTransitionTo(next) {
		return nil, domain.ErrInvalidStatusOrder
	}

	if err := u.orderRepo.UpdateStatus(ctx, orderID, next, nil, nil); err != nil {
		return nil, err
	}

	order.Advance(next)
	return order, nil
}

func (u *OrderUsecase) fetchAndValidateItems(ctx context.Context, in CreateOrderInput) ([]domain.MenuItem, error) {
	itemIDs := make([]uuid.UUID, 0, len(in.Items))
	quantities := make(map[uuid.UUID]int, len(in.Items))
	expectedPrices := make(map[uuid.UUID]float64, len(in.Items))

	for _, it := range in.Items {
		itemIDs = append(itemIDs, it.MenuItemID)
		quantities[it.MenuItemID] = it.Quantity
		expectedPrices[it.MenuItemID] = it.ExpectedPrice
	}

	menuItems, err := u.menuRepo.GetItemsByIDs(ctx, itemIDs)
	if err != nil {
		return nil, err
	}
	if len(menuItems) != len(itemIDs) {
		return nil, domain.ErrMenuItemNotFound
	}

	var unavailable []uuid.UUID
	var discrepancies []PriceDiscrepancy

	for _, item := range menuItems {
		if item.RestaurantID != in.RestaurantID {
			return nil, domain.ErrForeignRestaurantItem
		}
		if !item.IsAvailable {
			unavailable = append(unavailable, item.ID)
		}
		if exp := expectedPrices[item.ID]; item.Price != exp {
			discrepancies = append(discrepancies, PriceDiscrepancy{
				MenuItemID:    item.ID,
				ExpectedPrice: exp,
				ActualPrice:   item.Price,
			})
		}
	}

	if len(unavailable) > 0 || len(discrepancies) > 0 {
		return nil, &OrderConflictError{
			UnavailableItemIDs: unavailable,
			PriceDiscrepancies: discrepancies,
		}
	}

	return menuItems, nil
}

func (u *OrderUsecase) assembleOrder(in CreateOrderInput, menuItems []domain.MenuItem) *domain.Order {
	orderID := uuid.New()
	now := time.Now().UTC()

	quantities := make(map[uuid.UUID]int, len(in.Items))
	for _, it := range in.Items {
		quantities[it.MenuItemID] = it.Quantity
	}

	var totalAmount float64
	snapshots := make([]domain.OrderItemSnapshot, len(menuItems))

	for i, item := range menuItems {
		qty := quantities[item.ID]
		itemTotal := item.Price * float64(qty)
		totalAmount += itemTotal

		itemIDCopy := item.ID
		snapshots[i] = domain.OrderItemSnapshot{
			ID:           uuid.New(),
			OrderID:      orderID,
			MenuItemID:   &itemIDCopy,
			NameAtOrder:  item.Name,
			PriceAtOrder: item.Price,
			Quantity:     qty,
			TotalPrice:   itemTotal,
		}
	}

	return &domain.Order{
		ID:                     orderID,
		UserID:                 in.UserID,
		RestaurantID:           in.RestaurantID,
		Status:                 domain.OrderStatusCreated,
		DeliveryAddress:        in.DeliveryAddress,
		ContactPhone:           in.ContactPhone,
		Comment:                in.Comment,
		TotalAmount:            totalAmount,
		ConfirmationDeadlineAt: now.Add(10 * time.Minute),
		Items:                  snapshots,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

func (u *OrderUsecase) notifyRestaurant(rest *domain.Restaurant, order *domain.Order) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := u.webhookSender.SendOrderCreated(ctx, rest, order); err != nil {
		slog.Error("Failed to send order webhook",
			"order_id", order.ID,
			"restaurant_id", rest.ID,
			"error", err,
		)
	}
}
