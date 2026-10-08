package http

import (
	"food-delivery-api/internal/core/delivery/http/middleware"
	v1 "food-delivery-api/internal/core/delivery/http/v1"
	"food-delivery-api/internal/core/domain"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type Handlers struct {
	ClientHandler   *v1.ClientHandler
	PartnerHandler  *v1.PartnerHandler
	PlatformHandler *v1.PlatformHandler
	RestaurantRepo  domain.RestaurantRepository
}

func NewRouter(h Handlers) *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.ClientIPFromHeader("X-Real-IP"))
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/restaurants", h.ClientHandler.ListRestaurants)
		r.Get("/restaurants/{restaurant_id}/menu", h.ClientHandler.GetRestaurantMenu)

		r.Group(func(r chi.Router) {
			r.Use(middleware.UserAuthMiddleware)
			r.Post("/orders", h.ClientHandler.CreateOrder)
			r.Get("/orders/{order_id}", h.ClientHandler.GetOrder)
			r.Post("/orders/{order_id}/cancel", h.ClientHandler.CancelOrder)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.PartnerAuthMiddleware(h.RestaurantRepo))
			r.Post("/partner/orders/{order_id}/accept", h.PartnerHandler.AcceptOrder)
			r.Post("/partner/orders/{order_id}/reject", h.PartnerHandler.RejectOrder)
			r.Patch("/partner/orders/{order_id}/status", h.PartnerHandler.UpdateKitchenStatus)
			r.Patch("/partner/menu/items/{item_id}", h.PartnerHandler.UpdateMenuItemAvailability)
		})

		r.Post("/internal/orders/{order_id}/delivery-step", h.PlatformHandler.DeliveryStep)
	})

	RegisterSwaggerRoutes(r)

	return r
}
