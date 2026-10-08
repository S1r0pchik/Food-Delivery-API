package usecase_test

import (
	"context"
	"testing"

	"food-delivery-api/internal/core/domain"
	"food-delivery-api/internal/core/usecase"

	"github.com/google/uuid"
)

func TestCatalogUsecase_GetRestaurantMenu_Success(t *testing.T) {
	restID := uuid.New()
	expectedRest := &domain.Restaurant{ID: restID, Name: "Додо & Ко", IsOpen: true}
	expectedCategories := []domain.MenuCategory{
		{ID: uuid.New(), RestaurantID: restID, Name: "Пицца"},
	}

	restRepo := &mockRestaurantRepo{restaurant: expectedRest}
	menuRepo := &mockMenuRepoCategories{categories: expectedCategories}

	uc := usecase.NewCatalogUsecase(restRepo, menuRepo)

	rest, menu, err := uc.GetRestaurantMenu(context.Background(), restID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rest.ID != restID {
		t.Errorf("expected restaurant id %s, got %s", restID, rest.ID)
	}
	if len(menu) != 1 || menu[0].Name != "Пицца" {
		t.Errorf("expected 1 category 'Пицца', got %+v", menu)
	}
}

type mockMenuRepoCategories struct {
	mockMenuRepo
	categories []domain.MenuCategory
}

func (m *mockMenuRepoCategories) GetMenuByRestaurantID(_ context.Context, _ uuid.UUID) ([]domain.MenuCategory, error) {
	return m.categories, nil
}

func TestCatalogUsecase_ListRestaurants(t *testing.T) {
	restID := uuid.New()
	restRepo := &mockRestaurantRepo{restaurant: &domain.Restaurant{ID: restID, Name: "Пиццерия", IsOpen: true}}
	menuRepo := &mockMenuRepo{}

	uc := usecase.NewCatalogUsecase(restRepo, menuRepo)

	list, err := uc.ListRestaurants(context.Background(), true, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 restaurant, got %d", len(list))
	}
}

