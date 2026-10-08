package usecase

import (
	"context"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
)

type CatalogUsecase struct {
	restaurantRepo domain.RestaurantRepository
	menuRepo       domain.MenuRepository
}

func NewCatalogUsecase(restaurantRepo domain.RestaurantRepository, menuRepo domain.MenuRepository) *CatalogUsecase {
	return &CatalogUsecase{
		restaurantRepo: restaurantRepo,
		menuRepo:       menuRepo,
	}
}

func (u *CatalogUsecase) ListRestaurants(ctx context.Context, openOnly bool, limit, offset int) ([]domain.Restaurant, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return u.restaurantRepo.List(ctx, openOnly, limit, offset)
}

func (u *CatalogUsecase) GetRestaurantMenu(ctx context.Context, restaurantID uuid.UUID) (*domain.Restaurant, []domain.MenuCategory, error) {
	restaurant, err := u.restaurantRepo.GetByID(ctx, restaurantID)
	if err != nil {
		return nil, nil, err
	}

	menu, err := u.menuRepo.GetMenuByRestaurantID(ctx, restaurantID)
	if err != nil {
		return nil, nil, err
	}

	return restaurant, menu, nil
}
