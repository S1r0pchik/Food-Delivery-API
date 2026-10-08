package client_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"food-delivery-api/internal/restaurant/client"
)

func TestCoreClient_ResponseErrors(t *testing.T) {
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

			c := client.NewCoreClient(srv.URL, "key")
			err := c.AcceptOrder(context.Background(), "order-id", 20)
			if status == http.StatusOK {
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("expected incomplete response error, got %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "status 500") {
				t.Fatalf("expected HTTP status error, got %v", err)
			}
		})
	}
}

func TestCoreClient_AllMethods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "valid-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := client.NewCoreClient(srv.URL, "valid-key")
	ctx := context.Background()

	if err := c.AcceptOrder(ctx, "id-1", 20); err != nil {
		t.Errorf("AcceptOrder failed: %v", err)
	}
	if err := c.RejectOrder(ctx, "id-1", "REJECTED_BY_RESTAURANT"); err != nil {
		t.Errorf("RejectOrder failed: %v", err)
	}
	if err := c.UpdateKitchenStatus(ctx, "id-1", "COOKING"); err != nil {
		t.Errorf("UpdateKitchenStatus failed: %v", err)
	}
	if err := c.UpdateItemAvailability(ctx, "id-1", true); err != nil {
		t.Errorf("UpdateItemAvailability failed: %v", err)
	}
}
