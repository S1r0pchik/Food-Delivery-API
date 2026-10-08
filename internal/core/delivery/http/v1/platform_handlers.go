package v1

import (
	"errors"
	"net/http"

	"food-delivery-api/internal/core/delivery/http/dto"
	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"
	"food-delivery-api/pkg/response"
)

type PlatformHandler struct {
	orderUsecase *usecase.OrderUsecase
}

func NewPlatformHandler(orderUsecase *usecase.OrderUsecase) *PlatformHandler {
	return &PlatformHandler{orderUsecase: orderUsecase}
}

func (h *PlatformHandler) DeliveryStep(w http.ResponseWriter, r *http.Request) {
	orderID, ok := parseUUIDParam(w, r, "order_id")
	if !ok {
		return
	}

	order, err := h.orderUsecase.AdvanceDeliveryStep(r.Context(), orderID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrOrderNotFound):
			response.Error(w, http.StatusNotFound, "NOT_FOUND", "Order not found")
		case errors.Is(err, domain.ErrInvalidStatusOrder):
			response.Error(w, http.StatusBadRequest, "INVALID_STATUS", "Order must be in READY_FOR_PICKUP or IN_DELIVERY status")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to advance delivery step")
		}
		return
	}

	response.OK(w, dto.NewOrderResponse(order))
}
