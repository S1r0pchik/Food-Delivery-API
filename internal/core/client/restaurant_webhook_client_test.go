package client_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"food-delivery-api/internal/core/client"
	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
)

func TestRestaurantWebhookClient_ResponseErrors(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "10")
				w.WriteHeader(status)
				if _, err := w.Write([]byte("short")); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer srv.Close()

			c := client.NewRestaurantWebhookClient()
			err := c.SendOrderCreated(context.Background(),
				&domain.Restaurant{WebhookURL: srv.URL, WebhookSecret: "secret"},
				&domain.Order{ID: uuid.New(), CreatedAt: time.Now().UTC()},
			)
			if status == http.StatusOK {
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("expected incomplete response error, got %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "status: 500") {
				t.Fatalf("expected HTTP status error, got %v", err)
			}
		})
	}
}

func TestRestaurantWebhookClient_SendOrderCreated(t *testing.T) {
	secret := "my-shared-secret-key"
	var receivedSig string
	var receivedEventID string
	var bodyBytes []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Signature")
		receivedEventID = r.Header.Get("X-Event-ID")
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := client.NewRestaurantWebhookClient()

	restaurant := &domain.Restaurant{
		WebhookURL:    srv.URL,
		WebhookSecret: secret,
	}

	order := &domain.Order{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		Items: []domain.OrderItemSnapshot{
			{NameAtOrder: "Пепперони", Quantity: 1},
		},
	}

	err := c.SendOrderCreated(context.Background(), restaurant, order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedEventID == "" {
		t.Error("expected non-empty X-Event-ID header")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(bodyBytes)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if receivedSig != expectedSig {
		t.Errorf("HMAC mismatch: got %s, want %s", receivedSig, expectedSig)
	}
}
