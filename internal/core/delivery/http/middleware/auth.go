package middleware

import (
	"context"
	"net/http"

	"food-delivery-api/internal/core/domain"
	"food-delivery-api/pkg/response"

	"github.com/google/uuid"
)

type contextKey string

const (
	userIDContextKey     contextKey = "ctx_user_id"
	restaurantContextKey contextKey = "ctx_restaurant"
)

func UserAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userIDStr := r.Header.Get("X-User-ID")
		if userIDStr == "" {
			response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing X-User-ID header")
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_USER_ID", "X-User-ID must be a valid UUID")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	val, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	return val, ok
}

func PartnerAuthMiddleware(repo domain.RestaurantRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing X-API-Key header")
				return
			}

			restaurant, err := repo.GetByAPIKey(r.Context(), apiKey)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), restaurantContextKey, restaurant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRestaurantFromContext(ctx context.Context) (*domain.Restaurant, bool) {
	val, ok := ctx.Value(restaurantContextKey).(*domain.Restaurant)
	return val, ok
}
