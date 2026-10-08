package worker

import (
	"context"
	"testing"
	"time"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
)

type mockTimeoutOrderRepo struct {
	expiredOrders []domain.Order
	cancelledIDs  []uuid.UUID
}

func (m *mockTimeoutOrderRepo) Create(_ context.Context, _ *domain.Order) error { return nil }
func (m *mockTimeoutOrderRepo) GetByID(_ context.Context, _ uuid.UUID) (*domain.Order, error) {
	return nil, nil
}
func (m *mockTimeoutOrderRepo) UpdateStatus(_ context.Context, id uuid.UUID, status domain.OrderStatus, reason *domain.CancellationReason, _ *int) error {
	if status == domain.OrderStatusCancelled && reason != nil && *reason == domain.CancellationReasonTimeout {
		m.cancelledIDs = append(m.cancelledIDs, id)
	}
	return nil
}
func (m *mockTimeoutOrderRepo) GetExpiredCreatedOrders(_ context.Context, _ time.Time, _ int) ([]domain.Order, error) {
	return m.expiredOrders, nil
}

func TestTimeoutWorker_ProcessExpiredOrders(t *testing.T) {
	orderID := uuid.New()
	repo := &mockTimeoutOrderRepo{
		expiredOrders: []domain.Order{
			{ID: orderID, Status: domain.OrderStatusCreated},
		},
	}

	w := NewTimeoutWorker(repo, 10*time.Millisecond)

	w.processExpiredOrders(context.Background())

	if len(repo.cancelledIDs) != 1 || repo.cancelledIDs[0] != orderID {
		t.Fatalf("expected order %s to be cancelled, got %v", orderID, repo.cancelledIDs)
	}

	repo.expiredOrders = nil
	repo.cancelledIDs = nil
	w.processExpiredOrders(context.Background())
	if len(repo.cancelledIDs) != 0 {
		t.Fatalf("expected 0 cancelled orders for empty batch")
	}
}

func TestTimeoutWorker_Start_Shutdown(t *testing.T) {
	repo := &mockTimeoutOrderRepo{}
	w := NewTimeoutWorker(repo, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	w.Start(ctx)
}
