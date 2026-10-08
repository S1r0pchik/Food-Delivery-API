package v1

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"food-delivery-api/internal/core/delivery/http/dto"
	"food-delivery-api/internal/core/delivery/http/middleware"
	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"
	"food-delivery-api/pkg/response"
)

type PartnerHandler struct {
	partnerUsecase *usecase.PartnerUsecase
}

func NewPartnerHandler(partnerUsecase *usecase.PartnerUsecase) *PartnerHandler {
	return &PartnerHandler{partnerUsecase: partnerUsecase}
}

func (h *PartnerHandler) AcceptOrder(w http.ResponseWriter, r *http.Request) {
	restaurant, _ := middleware.GetRestaurantFromContext(r.Context())
	orderID, ok := parseUUIDParam(w, r, "order_id")
	if !ok {
		return
	}

	var req dto.AcceptOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	if req.CookingTimeMinutes <= 0 || req.CookingTimeMinutes > 180 {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Cooking time must be between 1 and 180 minutes")
		return
	}

	order, err := h.partnerUsecase.AcceptOrder(r.Context(), restaurant.ID, orderID, req.CookingTimeMinutes)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found for this restaurant")
		case errors.Is(err, domain.ErrInvalidStatusOrder):
			response.Error(w, http.StatusBadRequest, "INVALID_STATUS", "Order is not in CREATED status")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to accept order")
		}
		return
	}

	response.OK(w, dto.NewOrderResponse(order))
}

func (h *PartnerHandler) RejectOrder(w http.ResponseWriter, r *http.Request) {
	restaurant, _ := middleware.GetRestaurantFromContext(r.Context())
	orderID, ok := parseUUIDParam(w, r, "order_id")
	if !ok {
		return
	}

	var req dto.RejectOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	if req.Comment != nil {
		slog.Info("Restaurant rejected order with comment",
			"restaurant_id", restaurant.ID,
			"order_id", orderID,
			"comment", *req.Comment,
		)
	}

	reason := domain.CancellationReasonRejectedByRestaurant
	order, err := h.partnerUsecase.RejectOrder(r.Context(), restaurant.ID, orderID, reason)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found for this restaurant")
		case errors.Is(err, domain.ErrInvalidStatusOrder):
			response.Error(w, http.StatusBadRequest, "INVALID_STATUS", "Order cannot be rejected at this stage")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to reject order")
		}
		return
	}

	response.OK(w, dto.NewOrderResponse(order))
}

func (h *PartnerHandler) UpdateKitchenStatus(w http.ResponseWriter, r *http.Request) {
	restaurant, _ := middleware.GetRestaurantFromContext(r.Context())
	orderID, ok := parseUUIDParam(w, r, "order_id")
	if !ok {
		return
	}

	var req dto.UpdateKitchenStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	order, err := h.partnerUsecase.UpdateKitchenStatus(r.Context(), restaurant.ID, orderID, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found for this restaurant")
		case errors.Is(err, domain.ErrInvalidStatusOrder):
			response.Error(w, http.StatusBadRequest, "INVALID_STATUS", "Invalid kitchen status transition")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update kitchen status")
		}
		return
	}

	response.OK(w, dto.NewOrderResponse(order))
}

func (h *PartnerHandler) UpdateMenuItemAvailability(w http.ResponseWriter, r *http.Request) {
	restaurant, _ := middleware.GetRestaurantFromContext(r.Context())
	itemID, ok := parseUUIDParam(w, r, "item_id")
	if !ok {
		return
	}

	var req dto.UpdateMenuItemAvailabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	item, err := h.partnerUsecase.UpdateItemAvailability(r.Context(), restaurant.ID, itemID, req.IsAvailable)
	if err != nil {
		if errors.Is(err, domain.ErrMenuItemNotFound) {
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Menu item not found for this restaurant")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update item availability")
		return
	}

	response.OK(w, dto.NewMenuItemResponse(*item))
}
