package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"

	"github.com/google/uuid"
)

type mockRestaurantRepo struct {
	restaurant *domain.Restaurant
}

func (m *mockRestaurantRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Restaurant, error) {
	return m.restaurant, nil
}
func (m *mockRestaurantRepo) GetByAPIKey(_ context.Context, _ string) (*domain.Restaurant, error) {
	return nil, nil
}
func (m *mockRestaurantRepo) List(_ context.Context, _ bool, _, _ int) ([]domain.Restaurant, error) {
	if m.restaurant != nil {
		return []domain.Restaurant{*m.restaurant}, nil
	}
	return nil, nil
}

type mockMenuRepo struct {
	items []domain.MenuItem
}

func (m *mockMenuRepo) GetMenuByRestaurantID(_ context.Context, _ uuid.UUID) ([]domain.MenuCategory, error) {
	return nil, nil
}
func (m *mockMenuRepo) GetItemsByIDs(_ context.Context, _ []uuid.UUID) ([]domain.MenuItem, error) {
	return m.items, nil
}
func (m *mockMenuRepo) UpdateItemAvailability(_ context.Context, itemID uuid.UUID, isAvail bool) (*domain.MenuItem, error) {
	for i := range m.items {
		if m.items[i].ID == itemID {
			m.items[i].IsAvailable = isAvail
			return &m.items[i], nil
		}
	}
	return nil, domain.ErrMenuItemNotFound
}

type mockOrderRepo struct {
	savedOrder *domain.Order
}

func (m *mockOrderRepo) Create(_ context.Context, order *domain.Order) error {
	m.savedOrder = order
	return nil
}
func (m *mockOrderRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Order, error) {
	return m.savedOrder, nil
}
func (m *mockOrderRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status domain.OrderStatus, reason *domain.CancellationReason, cookingTime *int) error {
	if m.savedOrder != nil {
		m.savedOrder.Status = status
		m.savedOrder.CancellationReason = reason
		m.savedOrder.EstimatedCookingTimeMinutes = cookingTime
	}
	return nil
}
func (m *mockOrderRepo) GetExpiredCreatedOrders(_ context.Context, _ time.Time, _ int) ([]domain.Order, error) {
	return nil, nil
}

type mockWebhookSender struct{}

func (m *mockWebhookSender) SendOrderCreated(_ context.Context, _ *domain.Restaurant, _ *domain.Order) error {
	return nil
}

func TestOrderUsecase_CreateOrder_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*usecase.CreateOrderInput)
		wantErr error
		message string
	}{
		{"empty cart", func(in *usecase.CreateOrderInput) { in.Items = nil }, domain.ErrEmptyOrderItems, "at least one item"},
		{"empty address", func(in *usecase.CreateOrderInput) { in.DeliveryAddress = "" }, domain.ErrInvalidOrder, "delivery_address"},
		{"blank address", func(in *usecase.CreateOrderInput) { in.DeliveryAddress = " \t\n" }, domain.ErrInvalidOrder, "delivery_address"},
		{"empty phone", func(in *usecase.CreateOrderInput) { in.ContactPhone = "" }, domain.ErrInvalidOrder, "contact_phone"},
		{"blank phone", func(in *usecase.CreateOrderInput) { in.ContactPhone = " \t\n" }, domain.ErrInvalidOrder, "contact_phone"},
		{"zero quantity", func(in *usecase.CreateOrderInput) { in.Items[0].Quantity = 0 }, domain.ErrInvalidOrder, "quantity must be positive"},
		{"negative quantity", func(in *usecase.CreateOrderInput) { in.Items[0].Quantity = -1 }, domain.ErrInvalidOrder, "quantity must be positive"},
		{"duplicate item", func(in *usecase.CreateOrderInput) {
			duplicate := in.Items[0]
			duplicate.Quantity = 2
			in.Items = append(in.Items, duplicate)
		}, domain.ErrInvalidOrder, "duplicate item"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := usecase.CreateOrderInput{
				UserID:          uuid.New(),
				RestaurantID:    uuid.New(),
				DeliveryAddress: "ул. Мира, 1",
				ContactPhone:    "+79991234567",
				Items: []usecase.CreateOrderItemInput{
					{MenuItemID: uuid.New(), Quantity: 1, ExpectedPrice: 590},
				},
			}
			tt.change(&in)

			// Invalid input must be rejected without accessing repositories or sending webhooks.
			uc := usecase.NewOrderUsecase(nil, nil, nil, nil)
			order, err := uc.CreateOrder(context.Background(), in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
			if order != nil {
				t.Fatal("invalid input returned an order")
			}
			if !strings.Contains(err.Error(), tt.message) {
				t.Errorf("expected message containing %q, got %q", tt.message, err.Error())
			}
		})
	}
}

func TestOrderUsecase_CreateOrder_Success(t *testing.T) {
	restID := uuid.New()
	itemID := uuid.New()

	restRepo := &mockRestaurantRepo{
		restaurant: &domain.Restaurant{ID: restID, IsOpen: true},
	}
	menuRepo := &mockMenuRepo{
		items: []domain.MenuItem{
			{ID: itemID, RestaurantID: restID, Name: "Пицца Маргарита", Price: 500.0, IsAvailable: true},
		},
	}
	orderRepo := &mockOrderRepo{}
	uc := usecase.NewOrderUsecase(orderRepo, menuRepo, restRepo, &mockWebhookSender{})

	input := usecase.CreateOrderInput{
		UserID:          uuid.New(),
		RestaurantID:    restID,
		DeliveryAddress: "ул. Пушкина, д. Колотушкина",
		ContactPhone:    "+79990001122",
		Items: []usecase.CreateOrderItemInput{
			{MenuItemID: itemID, Quantity: 2, ExpectedPrice: 500.0},
		},
	}

	order, err := uc.CreateOrder(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if order.Status != domain.OrderStatusCreated {
		t.Errorf("expected status %s, got %s", domain.OrderStatusCreated, order.Status)
	}
	if order.TotalAmount != 1000.0 {
		t.Errorf("expected total amount 1000.0, got %f", order.TotalAmount)
	}
	if len(order.Items) != 1 || order.Items[0].NameAtOrder != "Пицца Маргарита" {
		t.Errorf("snapshot was not captured properly: %+v", order.Items)
	}
}

func TestOrderUsecase_CreateOrder_StopListConflict(t *testing.T) {
	restID := uuid.New()
	itemID := uuid.New()

	restRepo := &mockRestaurantRepo{
		restaurant: &domain.Restaurant{ID: restID, IsOpen: true},
	}
	menuRepo := &mockMenuRepo{
		items: []domain.MenuItem{
			{ID: itemID, RestaurantID: restID, Name: "Пицца Трюфельная", Price: 900.0, IsAvailable: false},
		},
	}
	orderRepo := &mockOrderRepo{}
	uc := usecase.NewOrderUsecase(orderRepo, menuRepo, restRepo, &mockWebhookSender{})

	input := usecase.CreateOrderInput{
		UserID:          uuid.New(),
		RestaurantID:    restID,
		DeliveryAddress: "ул. Ленина, д. 1",
		ContactPhone:    "+79990001122",
		Items: []usecase.CreateOrderItemInput{
			{MenuItemID: itemID, Quantity: 1, ExpectedPrice: 900.0},
		},
	}

	_, err := uc.CreateOrder(context.Background(), input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	conflictErr, ok := err.(*usecase.OrderConflictError)
	if !ok {
		t.Fatalf("expected OrderConflictError, got %T (%v)", err, err)
	}

	if len(conflictErr.UnavailableItemIDs) != 1 || conflictErr.UnavailableItemIDs[0] != itemID {
		t.Errorf("expected item %s in unavailable list, got %v", itemID, conflictErr.UnavailableItemIDs)
	}
}

func TestOrderUsecase_CancelAndDeliveryFlow(t *testing.T) {
	userID := uuid.New()
	otherUserID := uuid.New()
	orderID := uuid.New()

	order := &domain.Order{
		ID:        orderID,
		UserID:    userID,
		Status:    domain.OrderStatusCreated,
		CreatedAt: time.Now().UTC(),
	}

	orderRepo := &mockOrderRepo{savedOrder: order}
	uc := usecase.NewOrderUsecase(orderRepo, &mockMenuRepo{}, &mockRestaurantRepo{}, &mockWebhookSender{})

	found, err := uc.GetOrderByID(context.Background(), orderID)
	if err != nil || found.ID != orderID {
		t.Fatalf("GetOrderByID failed: %v", err)
	}

	if _, err = uc.CancelOrderByUser(context.Background(), orderID, otherUserID); err != domain.ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound for wrong user, got %v", err)
	}

	cancelled, err := uc.CancelOrderByUser(context.Background(), orderID, userID)
	if err != nil || cancelled.Status != domain.OrderStatusCancelled {
		t.Fatalf("CancelOrderByUser failed: %v", err)
	}

	if _, err = uc.CancelOrderByUser(context.Background(), orderID, userID); err != domain.ErrCannotCancelOrder {
		t.Errorf("expected ErrCannotCancelOrder, got %v", err)
	}

	order.Status = domain.OrderStatusReadyForPickup
	step1, err := uc.AdvanceDeliveryStep(context.Background(), orderID)
	if err != nil || step1.Status != domain.OrderStatusInDelivery {
		t.Errorf("expected IN_DELIVERY, got %v, err %v", step1, err)
	}

	step2, err := uc.AdvanceDeliveryStep(context.Background(), orderID)
	if err != nil || step2.Status != domain.OrderStatusDelivered {
		t.Errorf("expected DELIVERED, got %v, err %v", step2, err)
	}

	if _, err = uc.AdvanceDeliveryStep(context.Background(), orderID); err != domain.ErrInvalidStatusOrder {
		t.Errorf("expected ErrInvalidStatusOrder, got %v", err)
	}
}

func TestOrderConflictError_Error(t *testing.T) {
	err := &usecase.OrderConflictError{
		UnavailableItemIDs: []uuid.UUID{uuid.New()},
	}
	if err.Error() == "" {
		t.Error("expected non-empty error string")
	}
}
