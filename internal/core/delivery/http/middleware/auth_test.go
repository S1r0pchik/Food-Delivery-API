package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"food-delivery-api/internal/core/delivery/http/middleware"
	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
)

func TestUserAuthMiddleware(t *testing.T) {
	validUserID := uuid.New()

	tests := []struct {
		name           string
		headerValue    string
		expectedStatus int
	}{
		{
			name:           "Нет заголовка X-User-ID",
			headerValue:    "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Невалидный UUID",
			headerValue:    "not-a-uuid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Валидный UUID",
			headerValue:    validUserID.String(),
			expectedStatus: http.StatusOK,
		},
	}

	handler := middleware.UserAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := middleware.GetUserIDFromContext(r.Context())
		if !ok || id != validUserID {
			t.Errorf("expected user id %s in context", validUserID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-User-ID", tt.headerValue)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d; want %d", rec.Code, tt.expectedStatus)
			}
		})
	}
}

type mockRestRepoForAuth struct {
	apiKey     string
	restaurant *domain.Restaurant
}

func (m *mockRestRepoForAuth) GetByID(_ context.Context, _ uuid.UUID) (*domain.Restaurant, error) {
	return nil, nil
}
func (m *mockRestRepoForAuth) GetByAPIKey(_ context.Context, key string) (*domain.Restaurant, error) {
	if key == m.apiKey {
		return m.restaurant, nil
	}
	return nil, domain.ErrRestaurantNotFound
}
func (m *mockRestRepoForAuth) List(_ context.Context, _ bool, _, _ int) ([]domain.Restaurant, error) {
	return nil, nil
}

func TestPartnerAuthMiddleware(t *testing.T) {
	repo := &mockRestRepoForAuth{
		apiKey:     "secret-token",
		restaurant: &domain.Restaurant{ID: uuid.New(), Name: "Пиццерия"},
	}

	handler := middleware.PartnerAuthMiddleware(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("missing api key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", rec.Code)
		}
	})

	t.Run("invalid api key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "wrong-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", rec.Code)
		}
	})

	t.Run("valid api key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "secret-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("want 200, got %d", rec.Code)
		}
	})
}

func TestGetRestaurantFromContext_Positive(t *testing.T) {
	expected := &domain.Restaurant{ID: uuid.New(), Name: "Тест"}
	repo := &mockRestRepoForAuth{
		apiKey:     "secret-token",
		restaurant: expected,
	}

	var captured *domain.Restaurant
	handler := middleware.PartnerAuthMiddleware(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = middleware.GetRestaurantFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "secret-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if captured == nil || captured.ID != expected.ID {
		t.Errorf("expected restaurant in context, got %v", captured)
	}
}
