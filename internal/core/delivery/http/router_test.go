package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	deliveryhttp "food-delivery-api/internal/core/delivery/http"
	"food-delivery-api/internal/core/delivery/http/middleware"
	v1 "food-delivery-api/internal/core/delivery/http/v1"
	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"

	"github.com/google/uuid"
)

type mockAllOrderRepo struct {
	order *domain.Order
}

func (m *mockAllOrderRepo) Create(_ context.Context, o *domain.Order) error {
	m.order = o
	return nil
}
func (m *mockAllOrderRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Order, error) {
	if m.order != nil {
		return m.order, nil
	}
	return nil, domain.ErrOrderNotFound
}
func (m *mockAllOrderRepo) UpdateStatus(_ context.Context, _ uuid.UUID, s domain.OrderStatus, r *domain.CancellationReason, cookingTime *int) error {
	if m.order != nil {
		m.order.Status = s
		m.order.CancellationReason = r
		m.order.EstimatedCookingTimeMinutes = cookingTime
	}
	return nil
}
func (m *mockAllOrderRepo) GetExpiredCreatedOrders(_ context.Context, _ time.Time, _ int) ([]domain.Order, error) {
	return nil, nil
}

type mockAllMenuRepo struct {
	items      []domain.MenuItem
	categories []domain.MenuCategory
}

func (m *mockAllMenuRepo) GetMenuByRestaurantID(_ context.Context, _ uuid.UUID) ([]domain.MenuCategory, error) {
	return m.categories, nil
}
func (m *mockAllMenuRepo) GetItemsByIDs(_ context.Context, _ []uuid.UUID) ([]domain.MenuItem, error) {
	return m.items, nil
}
func (m *mockAllMenuRepo) UpdateItemAvailability(_ context.Context, itemID uuid.UUID, isAvail bool) (*domain.MenuItem, error) {
	for i := range m.items {
		if m.items[i].ID == itemID {
			m.items[i].IsAvailable = isAvail
			return &m.items[i], nil
		}
	}
	return nil, domain.ErrMenuItemNotFound
}

type mockAllRestRepo struct {
	restaurant *domain.Restaurant
}

func (m *mockAllRestRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Restaurant, error) {
	return m.restaurant, nil
}
func (m *mockAllRestRepo) GetByAPIKey(_ context.Context, _ string) (*domain.Restaurant, error) {
	return m.restaurant, nil
}
func (m *mockAllRestRepo) List(_ context.Context, _ bool, _, _ int) ([]domain.Restaurant, error) {
	return []domain.Restaurant{*m.restaurant}, nil
}

type mockAllWebhookSender struct{}

func (m *mockAllWebhookSender) SendOrderCreated(_ context.Context, _ *domain.Restaurant, _ *domain.Order) error {
	return nil
}

func setupTestRouter() (http.Handler, *mockAllOrderRepo, *domain.Restaurant, *domain.MenuItem) {
	restID := uuid.New()
	itemID := uuid.New()

	restaurant := &domain.Restaurant{
		ID:            restID,
		Name:          "Тестовый Ресторан",
		IsOpen:        true,
		APIKey:        "partner-key",
		WebhookSecret: "secret",
	}

	item := domain.MenuItem{
		ID:           itemID,
		RestaurantID: restID,
		Name:         "Пицца",
		Price:        500,
		IsAvailable:  true,
	}

	orderRepo := &mockAllOrderRepo{}
	menuRepo := &mockAllMenuRepo{
		items: []domain.MenuItem{item},
		categories: []domain.MenuCategory{
			{ID: uuid.New(), RestaurantID: restID, Name: "Пицца", Items: []domain.MenuItem{item}},
		},
	}
	restRepo := &mockAllRestRepo{restaurant: restaurant}
	webhook := &mockAllWebhookSender{}

	orderUc := usecase.NewOrderUsecase(orderRepo, menuRepo, restRepo, webhook)
	partnerUc := usecase.NewPartnerUsecase(orderRepo, restRepo, menuRepo)
	catalogUc := usecase.NewCatalogUsecase(restRepo, menuRepo)

	handlers := deliveryhttp.Handlers{
		ClientHandler:   v1.NewClientHandler(catalogUc, orderUc),
		PartnerHandler:  v1.NewPartnerHandler(partnerUc),
		PlatformHandler: v1.NewPlatformHandler(orderUc),
		RestaurantRepo:  restRepo,
	}

	return deliveryhttp.NewRouter(handlers), orderRepo, restaurant, &item
}

func TestRouter_FullFlow(t *testing.T) {
	router, orderRepo, rest, item := setupTestRouter()
	userID := uuid.New()

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants?open_only=true&limit=10&offset=0", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("GET /restaurants code = %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/"+rest.ID.String()+"/menu", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /menu code = %d", rec2.Code)
	}

	body, _ := json.Marshal(map[string]any{
		"restaurant_id":    rest.ID,
		"delivery_address": "ул. Мира, 1",
		"contact_phone":    "+79991234567",
		"items": []map[string]any{
			{"item_id": item.ID, "quantity": 2, "expected_price": 500},
		},
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req3.Header.Set("X-User-ID", userID.String())
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusCreated {
		t.Fatalf("POST /orders code = %d", rec3.Code)
	}

	orderID := orderRepo.order.ID

	req4 := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String(), nil)
	req4.Header.Set("X-User-ID", userID.String())
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("GET /orders/{id} code = %d", rec4.Code)
	}

	accBody, _ := json.Marshal(map[string]any{"cooking_time_minutes": 25})
	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/partner/orders/"+orderID.String()+"/accept", bytes.NewReader(accBody))
	req5.Header.Set("X-API-Key", rest.APIKey)
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusOK {
		t.Fatalf("POST /partner/accept code = %d", rec5.Code)
	}

	for _, status := range []string{"COOKING", "READY_FOR_PICKUP"} {
		stBody, _ := json.Marshal(map[string]any{"status": status})
		reqSt := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/"+orderID.String()+"/status", bytes.NewReader(stBody))
		reqSt.Header.Set("X-API-Key", rest.APIKey)
		recSt := httptest.NewRecorder()
		router.ServeHTTP(recSt, reqSt)
		if recSt.Code != http.StatusOK {
			t.Fatalf("PATCH status %s code = %d", status, recSt.Code)
		}
	}

	for _, exp := range []domain.OrderStatus{domain.OrderStatusInDelivery, domain.OrderStatusDelivered} {
		reqDel := httptest.NewRequest(http.MethodPost, "/api/v1/internal/orders/"+orderID.String()+"/delivery-step", nil)
		recDel := httptest.NewRecorder()
		router.ServeHTTP(recDel, reqDel)
		if recDel.Code != http.StatusOK || orderRepo.order.Status != exp {
			t.Fatalf("delivery-step code = %d, status = %s", recDel.Code, orderRepo.order.Status)
		}
	}

	avBody, _ := json.Marshal(map[string]any{"is_available": false})
	reqAv := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/menu/items/"+item.ID.String(), bytes.NewReader(avBody))
	reqAv.Header.Set("X-API-Key", rest.APIKey)
	recAv := httptest.NewRecorder()
	router.ServeHTTP(recAv, reqAv)
	if recAv.Code != http.StatusOK {
		t.Fatalf("PATCH menu item code = %d", recAv.Code)
	}

	orderRepo.order.Status = domain.OrderStatusCreated
	rejBody, _ := json.Marshal(map[string]any{"reason": "REJECTED_BY_RESTAURANT"})
	reqRej := httptest.NewRequest(http.MethodPost, "/api/v1/partner/orders/"+orderID.String()+"/reject", bytes.NewReader(rejBody))
	reqRej.Header.Set("X-API-Key", rest.APIKey)
	recRej := httptest.NewRecorder()
	router.ServeHTTP(recRej, reqRej)
	if recRej.Code != http.StatusOK {
		t.Fatalf("POST /partner/reject code = %d", recRej.Code)
	}
}

func TestContextHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, ok := middleware.GetRestaurantFromContext(req.Context()); ok {
		t.Error("expected ok=false for empty context")
	}
}

func TestSwagger_Endpoints(t *testing.T) {
	router, _, _, _ := setupTestRouter()

	t.Run("Swagger UI Redirect", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("expected 301, got %d", rec.Code)
		}
	})

	t.Run("Swagger UI HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("expected text/html, got %s", ct)
		}
	})

	t.Run("Swagger OpenAPI YAML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/swagger/openapi.yaml", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/yaml; charset=utf-8" {
			t.Errorf("expected application/yaml, got %s", ct)
		}
		if len(rec.Body.Bytes()) == 0 {
			t.Error("expected non-empty openapi.yaml spec")
		}
	})
}

