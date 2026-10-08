package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	deliveryhttp "food-delivery-api/internal/core/delivery/http"
	v1 "food-delivery-api/internal/core/delivery/http/v1"
	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"

	"github.com/google/uuid"
)

type mockRepoBundle struct {
	order      *domain.Order
	restaurant *domain.Restaurant
	item       domain.MenuItem
}

func (m *mockRepoBundle) Create(_ context.Context, o *domain.Order) error {
	m.order = o
	return nil
}
func (m *mockRepoBundle) GetByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	if m.order != nil && m.order.ID == id {
		return m.order, nil
	}
	return nil, domain.ErrOrderNotFound
}
func (m *mockRepoBundle) UpdateStatus(_ context.Context, _ uuid.UUID, s domain.OrderStatus, r *domain.CancellationReason, c *int) error {
	if m.order != nil {
		m.order.Status = s
		m.order.CancellationReason = r
		m.order.EstimatedCookingTimeMinutes = c
	}
	return nil
}
func (m *mockRepoBundle) GetExpiredCreatedOrders(_ context.Context, _ time.Time, _ int) ([]domain.Order, error) {
	return nil, nil
}
func (m *mockRepoBundle) GetMenuByRestaurantID(_ context.Context, _ uuid.UUID) ([]domain.MenuCategory, error) {
	return []domain.MenuCategory{{ID: uuid.New(), Name: "Категория", Items: []domain.MenuItem{m.item}}}, nil
}
func (m *mockRepoBundle) GetItemsByIDs(_ context.Context, _ []uuid.UUID) ([]domain.MenuItem, error) {
	return []domain.MenuItem{m.item}, nil
}
func (m *mockRepoBundle) UpdateItemAvailability(_ context.Context, _ uuid.UUID, isAvail bool) (*domain.MenuItem, error) {
	m.item.IsAvailable = isAvail
	return &m.item, nil
}
func (m *mockRepoBundle) GetRestaurantByID(_ context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	if m.restaurant != nil && m.restaurant.ID == id {
		return m.restaurant, nil
	}
	return nil, domain.ErrRestaurantNotFound
}
func (m *mockRepoBundle) GetByAPIKey(_ context.Context, k string) (*domain.Restaurant, error) {
	if m.restaurant != nil && m.restaurant.APIKey == k {
		return m.restaurant, nil
	}
	return nil, domain.ErrRestaurantNotFound
}
func (m *mockRepoBundle) List(_ context.Context, _ bool, _, _ int) ([]domain.Restaurant, error) {
	if m.restaurant != nil {
		return []domain.Restaurant{*m.restaurant}, nil
	}
	return nil, nil
}

type restRepoAdapter struct{ *mockRepoBundle }

func (r restRepoAdapter) GetByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	return r.GetRestaurantByID(ctx, id)
}

type noopWebhook struct{}

func (noopWebhook) SendOrderCreated(_ context.Context, _ *domain.Restaurant, _ *domain.Order) error {
	return nil
}

func setupV1Router() (http.Handler, *mockRepoBundle) {
	restID := uuid.New()
	itemID := uuid.New()

	bundle := &mockRepoBundle{
		restaurant: &domain.Restaurant{
			ID:            restID,
			Name:          "Пиццерия",
			IsOpen:        true,
			APIKey:        "test-key",
			WebhookSecret: "secret",
		},
		item: domain.MenuItem{
			ID:           itemID,
			RestaurantID: restID,
			Name:         "Пепперони",
			Price:        600,
			IsAvailable:  true,
		},
	}

	orderUc := usecase.NewOrderUsecase(bundle, bundle, restRepoAdapter{bundle}, noopWebhook{})
	partnerUc := usecase.NewPartnerUsecase(bundle, restRepoAdapter{bundle}, bundle)
	catalogUc := usecase.NewCatalogUsecase(restRepoAdapter{bundle}, bundle)

	h := deliveryhttp.Handlers{
		ClientHandler:   v1.NewClientHandler(catalogUc, orderUc),
		PartnerHandler:  v1.NewPartnerHandler(partnerUc),
		PlatformHandler: v1.NewPlatformHandler(orderUc),
		RestaurantRepo:  restRepoAdapter{bundle},
	}

	return deliveryhttp.NewRouter(h), bundle
}

func TestV1_ListRestaurants_QueryParameters(t *testing.T) {
	tests := []struct {
		query   string
		status  int
		message string
	}{
		{"", http.StatusOK, ""},
		{"?open_only=false&limit=10&offset=0", http.StatusOK, ""},
		{"?open_only=maybe", http.StatusBadRequest, "open_only"},
		{"?limit=abc", http.StatusBadRequest, "limit"},
		{"?limit=999999999999999999999999999999", http.StatusBadRequest, "limit"},
		{"?offset=1.5", http.StatusBadRequest, "offset"},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			router, _ := setupV1Router()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants"+tt.query, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("expected status %d, got %d: %s", tt.status, rec.Code, rec.Body.String())
			}
			if tt.status == http.StatusBadRequest {
				var payload struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if payload.Code != "BAD_REQUEST" || !strings.Contains(payload.Message, tt.message) {
					t.Errorf("unexpected response: %+v", payload)
				}
			}
		})
	}
}

func TestV1_CreateOrder_InvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		change  func(map[string]any)
		message string
	}{
		{"blank address", func(body map[string]any) { body["delivery_address"] = " \t\n" }, "delivery_address"},
		{"missing phone", func(body map[string]any) { delete(body, "contact_phone") }, "contact_phone"},
		{"zero quantity", func(body map[string]any) {
			body["items"].([]map[string]any)[0]["quantity"] = 0
		}, "quantity must be positive"},
		{"duplicate item", func(body map[string]any) {
			items := body["items"].([]map[string]any)
			body["items"] = append(items, map[string]any{
				"item_id": items[0]["item_id"], "quantity": 2, "expected_price": items[0]["expected_price"],
			})
		}, "duplicate item"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, bundle := setupV1Router()
			body := map[string]any{
				"restaurant_id":    bundle.restaurant.ID,
				"delivery_address": "ул. Мира, 1",
				"contact_phone":    "+79991234567",
				"items": []map[string]any{
					{"item_id": bundle.item.ID, "quantity": 1, "expected_price": bundle.item.Price},
				},
			}
			tt.change(body)
			payload, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-User-ID", uuid.New().String())
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			var response struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Code != "BAD_REQUEST" || !strings.Contains(response.Message, tt.message) {
				t.Errorf("unexpected error response: %+v", response)
			}
			if bundle.order != nil {
				t.Fatal("invalid order was saved")
			}
		})
	}
}

func TestV1_HappyPathFlow(t *testing.T) {
	router, bundle := setupV1Router()
	userID := uuid.New()

	reqRest := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants?open_only=true&limit=10&offset=0", nil)
	recRest := httptest.NewRecorder()
	router.ServeHTTP(recRest, reqRest)
	if recRest.Code != http.StatusOK {
		t.Fatalf("GET /restaurants code = %d", recRest.Code)
	}

	reqMenu := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/"+bundle.restaurant.ID.String()+"/menu", nil)
	recMenu := httptest.NewRecorder()
	router.ServeHTTP(recMenu, reqMenu)
	if recMenu.Code != http.StatusOK {
		t.Fatalf("GET /menu code = %d", recMenu.Code)
	}

	body, _ := json.Marshal(map[string]any{
		"restaurant_id":    bundle.restaurant.ID,
		"delivery_address": "ул. Мира, 1",
		"contact_phone":    "+79991234567",
		"items": []map[string]any{
			{"item_id": bundle.item.ID, "quantity": 1, "expected_price": bundle.item.Price},
		},
	})
	reqOrder := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	reqOrder.Header.Set("X-User-ID", userID.String())
	recOrder := httptest.NewRecorder()
	router.ServeHTTP(recOrder, reqOrder)
	if recOrder.Code != http.StatusCreated {
		t.Fatalf("POST /orders code = %d", recOrder.Code)
	}

	orderID := bundle.order.ID

	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String(), nil)
	reqGet.Header.Set("X-User-ID", userID.String())
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET /orders/{id} code = %d", recGet.Code)
	}

	reqCancel := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+orderID.String()+"/cancel", nil)
	reqCancel.Header.Set("X-User-ID", userID.String())
	recCancel := httptest.NewRecorder()
	router.ServeHTTP(recCancel, reqCancel)
	if recCancel.Code != http.StatusOK {
		t.Fatalf("POST /orders/{id}/cancel code = %d", recCancel.Code)
	}

	bundle.order.Status = domain.OrderStatusCreated

	accBody, _ := json.Marshal(map[string]any{"cooking_time_minutes": 25})
	reqAcc := httptest.NewRequest(http.MethodPost, "/api/v1/partner/orders/"+orderID.String()+"/accept", bytes.NewReader(accBody))
	reqAcc.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	recAcc := httptest.NewRecorder()
	router.ServeHTTP(recAcc, reqAcc)
	if recAcc.Code != http.StatusOK {
		t.Fatalf("POST /partner/accept code = %d", recAcc.Code)
	}

	for _, status := range []string{"COOKING", "READY_FOR_PICKUP"} {
		stBody, _ := json.Marshal(map[string]any{"status": status})
		reqSt := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/"+orderID.String()+"/status", bytes.NewReader(stBody))
		reqSt.Header.Set("X-API-Key", bundle.restaurant.APIKey)
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
		if recDel.Code != http.StatusOK || bundle.order.Status != exp {
			t.Fatalf("delivery-step code = %d, status = %s", recDel.Code, bundle.order.Status)
		}
	}

	avBody, _ := json.Marshal(map[string]any{"is_available": false})
	reqAv := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/menu/items/"+bundle.item.ID.String(), bytes.NewReader(avBody))
	reqAv.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	recAv := httptest.NewRecorder()
	router.ServeHTTP(recAv, reqAv)
	if recAv.Code != http.StatusOK {
		t.Fatalf("PATCH menu item code = %d", recAv.Code)
	}

	bundle.order.Status = domain.OrderStatusCreated
	rejBody, _ := json.Marshal(map[string]any{"reason": "REJECTED_BY_RESTAURANT"})
	reqRej := httptest.NewRequest(http.MethodPost, "/api/v1/partner/orders/"+orderID.String()+"/reject", bytes.NewReader(rejBody))
	reqRej.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	recRej := httptest.NewRecorder()
	router.ServeHTTP(recRej, reqRej)
	if recRej.Code != http.StatusOK {
		t.Fatalf("POST /partner/reject code = %d", recRej.Code)
	}
}

func TestV1_InvalidRequestsAndEdgeCases(t *testing.T) {
	router, bundle := setupV1Router()
	userID := uuid.New()

	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/invalid-uuid/menu", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid menu UUID, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString("{broken-json"))
	req2.Header.Set("X-User-ID", userID.String())
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for broken json, got %d", rec2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/orders/not-a-uuid", nil)
	req3.Header.Set("X-User-ID", userID.String())
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid order UUID, got %d", rec3.Code)
	}

	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/orders/not-a-uuid/cancel", nil)
	req4.Header.Set("X-User-ID", userID.String())
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid order cancel UUID, got %d", rec4.Code)
	}

	req5 := httptest.NewRequest(http.MethodPost, "/api/v1/partner/orders/"+uuid.New().String()+"/accept", bytes.NewBufferString(`{"cooking_time_minutes":0}`))
	req5.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid cooking time, got %d", rec5.Code)
	}

	req6 := httptest.NewRequest(http.MethodPost, "/api/v1/internal/orders/invalid-uuid/delivery-step", nil)
	rec6 := httptest.NewRecorder()
	router.ServeHTTP(rec6, req6)
	if rec6.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid delivery step UUID, got %d", rec6.Code)
	}
}

func TestV1_DomainErrorsFlow(t *testing.T) {
	router, bundle := setupV1Router()
	userID := uuid.New()
	wrongUserID := uuid.New()

	bundle.item.IsAvailable = false
	conflictBody, _ := json.Marshal(map[string]any{
		"restaurant_id":    bundle.restaurant.ID,
		"delivery_address": "ул. Мира, 1",
		"contact_phone":    "+79991234567",
		"items": []map[string]any{
			{"item_id": bundle.item.ID, "quantity": 1, "expected_price": bundle.item.Price},
		},
	})
	reqConflict := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(conflictBody))
	reqConflict.Header.Set("X-User-ID", userID.String())
	recConflict := httptest.NewRecorder()
	router.ServeHTTP(recConflict, reqConflict)
	if recConflict.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", recConflict.Code)
	}

	reqNotFoundMenu := httptest.NewRequest(http.MethodGet, "/api/v1/restaurants/"+uuid.New().String()+"/menu", nil)
	recNotFoundMenu := httptest.NewRecorder()
	router.ServeHTTP(recNotFoundMenu, reqNotFoundMenu)
	if recNotFoundMenu.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing restaurant menu, got %d", recNotFoundMenu.Code)
	}

	reqNotFoundOrder := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+uuid.New().String(), nil)
	reqNotFoundOrder.Header.Set("X-User-ID", userID.String())
	recNotFoundOrder := httptest.NewRecorder()
	router.ServeHTTP(recNotFoundOrder, reqNotFoundOrder)
	if recNotFoundOrder.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing order, got %d", recNotFoundOrder.Code)
	}

	bundle.order = &domain.Order{
		ID:           uuid.New(),
		UserID:       userID,
		RestaurantID: bundle.restaurant.ID,
		Status:       domain.OrderStatusCreated,
	}
	reqForeignOrder := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+bundle.order.ID.String(), nil)
	reqForeignOrder.Header.Set("X-User-ID", wrongUserID.String())
	recForeignOrder := httptest.NewRecorder()
	router.ServeHTTP(recForeignOrder, reqForeignOrder)
	if recForeignOrder.Code != http.StatusNotFound {
		t.Errorf("expected 404 for foreign order, got %d", recForeignOrder.Code)
	}

	bundle.order.Status = domain.OrderStatusCooking
	reqCancelCooking := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+bundle.order.ID.String()+"/cancel", nil)
	reqCancelCooking.Header.Set("X-User-ID", userID.String())
	recCancelCooking := httptest.NewRecorder()
	router.ServeHTTP(recCancelCooking, reqCancelCooking)
	if recCancelCooking.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for cancelling cooking order, got %d", recCancelCooking.Code)
	}

	bundle.order.Status = domain.OrderStatusCreated
	reqInvalidStep := httptest.NewRequest(http.MethodPost, "/api/v1/internal/orders/"+bundle.order.ID.String()+"/delivery-step", nil)
	recInvalidStep := httptest.NewRecorder()
	router.ServeHTTP(recInvalidStep, reqInvalidStep)
	if recInvalidStep.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid delivery step status, got %d", recInvalidStep.Code)
	}
}

func TestV1_AdditionalErrorBranches(t *testing.T) {
	router, bundle := setupV1Router()
	orderID := uuid.New()

	bundle.order = &domain.Order{
		ID:           orderID,
		RestaurantID: bundle.restaurant.ID,
		Status:       domain.OrderStatusCreated,
	}

	reqInvalidReject := httptest.NewRequest(http.MethodPost, "/api/v1/partner/orders/invalid-uuid/reject", bytes.NewBufferString(`{"reason":"REJECTED_BY_RESTAURANT"}`))
	reqInvalidReject.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	recInvalidReject := httptest.NewRecorder()
	router.ServeHTTP(recInvalidReject, reqInvalidReject)
	if recInvalidReject.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid reject UUID, got %d", recInvalidReject.Code)
	}

	reqInvalidStatus := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/orders/invalid-uuid/status", bytes.NewBufferString(`{"status":"COOKING"}`))
	reqInvalidStatus.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	recInvalidStatus := httptest.NewRecorder()
	router.ServeHTTP(recInvalidStatus, reqInvalidStatus)
	if recInvalidStatus.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid status UUID, got %d", recInvalidStatus.Code)
	}

	reqInvalidItem := httptest.NewRequest(http.MethodPatch, "/api/v1/partner/menu/items/invalid-uuid", bytes.NewBufferString(`{"is_available":true}`))
	reqInvalidItem.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	recInvalidItem := httptest.NewRecorder()
	router.ServeHTTP(recInvalidItem, reqInvalidItem)
	if recInvalidItem.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid item UUID, got %d", recInvalidItem.Code)
	}

	bundle.order.Status = domain.OrderStatusDelivered
	accBody, _ := json.Marshal(map[string]any{"cooking_time_minutes": 20})
	reqInvalidAccept := httptest.NewRequest(http.MethodPost, "/api/v1/partner/orders/"+orderID.String()+"/accept", bytes.NewReader(accBody))
	reqInvalidAccept.Header.Set("X-API-Key", bundle.restaurant.APIKey)
	recInvalidAccept := httptest.NewRecorder()
	router.ServeHTTP(recInvalidAccept, reqInvalidAccept)
	if recInvalidAccept.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for accepting already delivered order, got %d", recInvalidAccept.Code)
	}
}
