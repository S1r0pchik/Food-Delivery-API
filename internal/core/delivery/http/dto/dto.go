package dto

import (
	"time"

	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"

	"github.com/google/uuid"
)

type CreateOrderItemRequest struct {
	MenuItemID    uuid.UUID `json:"item_id"`
	Quantity      int       `json:"quantity"`
	ExpectedPrice float64   `json:"expected_price"`
}

type CreateOrderRequest struct {
	RestaurantID    uuid.UUID                `json:"restaurant_id"`
	DeliveryAddress string                   `json:"delivery_address"`
	ContactPhone    string                   `json:"contact_phone"`
	Comment         *string                  `json:"comment,omitempty"`
	Items           []CreateOrderItemRequest `json:"items"`
}

func (r CreateOrderRequest) ToInput(userID uuid.UUID) usecase.CreateOrderInput {
	items := make([]usecase.CreateOrderItemInput, len(r.Items))
	for i, it := range r.Items {
		items[i] = usecase.CreateOrderItemInput{
			MenuItemID:    it.MenuItemID,
			Quantity:      it.Quantity,
			ExpectedPrice: it.ExpectedPrice,
		}
	}
	return usecase.CreateOrderInput{
		UserID:          userID,
		RestaurantID:    r.RestaurantID,
		DeliveryAddress: r.DeliveryAddress,
		ContactPhone:    r.ContactPhone,
		Comment:         r.Comment,
		Items:           items,
	}
}

type AcceptOrderRequest struct {
	CookingTimeMinutes int `json:"cooking_time_minutes"`
}

type RejectOrderRequest struct {
	Reason  domain.CancellationReason `json:"reason"`
	Comment *string                   `json:"comment,omitempty"`
}

type UpdateKitchenStatusRequest struct {
	Status domain.OrderStatus `json:"status"`
}

type UpdateMenuItemAvailabilityRequest struct {
	IsAvailable bool `json:"is_available"`
}

type RestaurantResponse struct {
	ID                          uuid.UUID `json:"id"`
	Name                        string    `json:"name"`
	Address                     string    `json:"address"`
	IsOpen                      bool      `json:"is_open"`
	CuisineType                 string    `json:"cuisine_type"`
	EstimatedCookingTimeMinutes int       `json:"estimated_cooking_time_minutes"`
}

func NewRestaurantResponse(rest domain.Restaurant) RestaurantResponse {
	return RestaurantResponse{
		ID:                          rest.ID,
		Name:                        rest.Name,
		Address:                     rest.Address,
		IsOpen:                      rest.IsOpen,
		CuisineType:                 rest.CuisineType,
		EstimatedCookingTimeMinutes: rest.EstimatedCookingTimeMinutes,
	}
}

func NewRestaurantResponses(restaurants []domain.Restaurant) []RestaurantResponse {
	res := make([]RestaurantResponse, len(restaurants))
	for i, rest := range restaurants {
		res[i] = NewRestaurantResponse(rest)
	}
	return res
}

type MenuItemResponse struct {
	ID           uuid.UUID `json:"id"`
	RestaurantID uuid.UUID `json:"restaurant_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Price        float64   `json:"price"`
	IsAvailable  bool      `json:"is_available"`
	PhotoURL     *string   `json:"photo_url,omitempty"`
}

func NewMenuItemResponse(it domain.MenuItem) MenuItemResponse {
	return MenuItemResponse{
		ID:           it.ID,
		RestaurantID: it.RestaurantID,
		Name:         it.Name,
		Description:  it.Description,
		Price:        it.Price,
		IsAvailable:  it.IsAvailable,
		PhotoURL:     it.PhotoURL,
	}
}

type MenuCategoryResponse struct {
	ID    uuid.UUID          `json:"id"`
	Name  string             `json:"name"`
	Items []MenuItemResponse `json:"items"`
}

type RestaurantMenuResponse struct {
	RestaurantID uuid.UUID              `json:"restaurant_id"`
	Categories   []MenuCategoryResponse `json:"categories"`
}

func NewMenuCategoryResponses(categories []domain.MenuCategory) []MenuCategoryResponse {
	catResponses := make([]MenuCategoryResponse, len(categories))
	for i, cat := range categories {
		items := make([]MenuItemResponse, len(cat.Items))
		for j, it := range cat.Items {
			items[j] = NewMenuItemResponse(it)
		}
		catResponses[i] = MenuCategoryResponse{
			ID:    cat.ID,
			Name:  cat.Name,
			Items: items,
		}
	}
	return catResponses
}

type OrderItemSnapshotResponse struct {
	MenuItemID *uuid.UUID `json:"item_id,omitempty"`
	Name       string     `json:"name"`
	Price      float64    `json:"price"`
	Quantity   int        `json:"quantity"`
	TotalPrice float64    `json:"total_price"`
}

type OrderDetailsResponse struct {
	ID                          uuid.UUID                   `json:"id"`
	UserID                      uuid.UUID                   `json:"user_id"`
	RestaurantID                uuid.UUID                   `json:"restaurant_id"`
	Status                      domain.OrderStatus          `json:"status"`
	CancellationReason          *domain.CancellationReason  `json:"cancellation_reason,omitempty"`
	DeliveryAddress             string                      `json:"delivery_address"`
	ContactPhone                string                      `json:"contact_phone"`
	Comment                     *string                     `json:"comment,omitempty"`
	TotalAmount                 float64                     `json:"total_amount"`
	EstimatedCookingTimeMinutes *int                        `json:"estimated_cooking_time_minutes,omitempty"`
	Items                       []OrderItemSnapshotResponse `json:"items"`
	CreatedAt                   time.Time                   `json:"created_at"`
	UpdatedAt                   time.Time                   `json:"updated_at"`
}

func NewOrderResponse(order *domain.Order) OrderDetailsResponse {
	items := make([]OrderItemSnapshotResponse, len(order.Items))
	for i, it := range order.Items {
		items[i] = OrderItemSnapshotResponse{
			MenuItemID: it.MenuItemID,
			Name:       it.NameAtOrder,
			Price:      it.PriceAtOrder,
			Quantity:   it.Quantity,
			TotalPrice: it.TotalPrice,
		}
	}

	return OrderDetailsResponse{
		ID:                          order.ID,
		UserID:                      order.UserID,
		RestaurantID:                order.RestaurantID,
		Status:                      order.Status,
		CancellationReason:          order.CancellationReason,
		DeliveryAddress:             order.DeliveryAddress,
		ContactPhone:                order.ContactPhone,
		Comment:                     order.Comment,
		TotalAmount:                 order.TotalAmount,
		EstimatedCookingTimeMinutes: order.EstimatedCookingTimeMinutes,
		Items:                       items,
		CreatedAt:                   order.CreatedAt,
		UpdatedAt:                   order.UpdatedAt,
	}
}

type OrderConflictResponse struct {
	Code               string             `json:"code"`
	Message            string             `json:"message"`
	UnavailableItemIDs []uuid.UUID        `json:"unavailable_items,omitempty"`
	PriceDiscrepancies []PriceDiscrepancy `json:"price_discrepancies,omitempty"`
}

type PriceDiscrepancy struct {
	MenuItemID    uuid.UUID `json:"item_id"`
	ExpectedPrice float64   `json:"expected_price"`
	ActualPrice   float64   `json:"actual_price"`
}

func NewOrderConflictResponse(conflictErr *usecase.OrderConflictError) OrderConflictResponse {
	discrepancies := make([]PriceDiscrepancy, len(conflictErr.PriceDiscrepancies))
	for i, pd := range conflictErr.PriceDiscrepancies {
		discrepancies[i] = PriceDiscrepancy{
			MenuItemID:    pd.MenuItemID,
			ExpectedPrice: pd.ExpectedPrice,
			ActualPrice:   pd.ActualPrice,
		}
	}

	return OrderConflictResponse{
		Code:               "ITEM_UNAVAILABLE_OR_PRICE_MISMATCH",
		Message:            "One or more items in the cart are out of stock or have changed price",
		UnavailableItemIDs: conflictErr.UnavailableItemIDs,
		PriceDiscrepancies: discrepancies,
	}
}
