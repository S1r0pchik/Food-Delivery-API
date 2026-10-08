package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"food-delivery-api/internal/core/delivery/http/dto"
	"food-delivery-api/internal/core/delivery/http/middleware"
	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"
	"food-delivery-api/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ClientHandler struct {
	catalogUsecase *usecase.CatalogUsecase
	orderUsecase   *usecase.OrderUsecase
}

func NewClientHandler(catalogUsecase *usecase.CatalogUsecase, orderUsecase *usecase.OrderUsecase) *ClientHandler {
	return &ClientHandler{
		catalogUsecase: catalogUsecase,
		orderUsecase:   orderUsecase,
	}
}

func (h *ClientHandler) ListRestaurants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	openOnly := true
	if val := q.Get("open_only"); val != "" {
		parsed, err := strconv.ParseBool(val)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "open_only must be a boolean")
			return
		}
		openOnly = parsed
	}

	var limit, offset int
	if val := q.Get("limit"); val != "" {
		parsed, err := strconv.Atoi(val)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "limit must be an integer")
			return
		}
		limit = parsed
	}
	if val := q.Get("offset"); val != "" {
		parsed, err := strconv.Atoi(val)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "offset must be an integer")
			return
		}
		offset = parsed
	}

	restaurants, err := h.catalogUsecase.ListRestaurants(r.Context(), openOnly, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch restaurants")
		return
	}

	response.OK(w, dto.NewRestaurantResponses(restaurants))
}

func (h *ClientHandler) GetRestaurantMenu(w http.ResponseWriter, r *http.Request) {
	restaurantID, ok := parseUUIDParam(w, r, "restaurant_id")
	if !ok {
		return
	}

	restaurant, categories, err := h.catalogUsecase.GetRestaurantMenu(r.Context(), restaurantID)
	if err != nil {
		if errors.Is(err, domain.ErrRestaurantNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Restaurant not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch menu")
		return
	}

	response.OK(w, dto.RestaurantMenuResponse{
		RestaurantID: restaurant.ID,
		Categories:   dto.NewMenuCategoryResponses(categories),
	})
}

func (h *ClientHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing user identity")
		return
	}

	var req dto.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	order, err := h.orderUsecase.CreateOrder(r.Context(), req.ToInput(userID))
	if err != nil {
		handleCreateOrderError(w, err)
		return
	}

	response.Created(w, dto.NewOrderResponse(order))
}

func (h *ClientHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())
	orderID, ok := parseUUIDParam(w, r, "order_id")
	if !ok {
		return
	}

	order, err := h.orderUsecase.GetOrderByID(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch order")
		return
	}

	if order.UserID != userID {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found")
		return
	}

	response.OK(w, dto.NewOrderResponse(order))
}

func (h *ClientHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.GetUserIDFromContext(r.Context())
	orderID, ok := parseUUIDParam(w, r, "order_id")
	if !ok {
		return
	}

	order, err := h.orderUsecase.CancelOrderByUser(r.Context(), orderID, userID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found")
		case errors.Is(err, domain.ErrCannotCancelOrder):
			response.Error(w, http.StatusBadRequest, "CANNOT_CANCEL", "Order cannot be cancelled at this stage")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to cancel order")
		}
		return
	}

	response.OK(w, dto.NewOrderResponse(order))
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid "+param+" UUID")
		return uuid.Nil, false
	}
	return id, true
}

func handleCreateOrderError(w http.ResponseWriter, err error) {
	var conflictErr *usecase.OrderConflictError
	if errors.As(err, &conflictErr) {
		response.JSON(w, http.StatusConflict, dto.NewOrderConflictResponse(conflictErr))
		return
	}

	switch {
	case errors.Is(err, domain.ErrRestaurantNotFound):
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "Restaurant not found")
	case errors.Is(err, domain.ErrRestaurantClosed):
		response.Error(w, http.StatusBadRequest, "RESTAURANT_CLOSED", "Restaurant is currently closed")
	case errors.Is(err, domain.ErrMenuItemNotFound):
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "One or more menu items not found")
	case errors.Is(err, domain.ErrForeignRestaurantItem):
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Menu item does not belong to the selected restaurant")
	case errors.Is(err, domain.ErrEmptyOrderItems):
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Cart cannot be empty")
	case errors.Is(err, domain.ErrInvalidOrder):
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create order")
	}
}
