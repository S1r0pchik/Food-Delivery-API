package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"food-delivery-api/internal/restaurant/client"
	"food-delivery-api/internal/restaurant/config"
	"food-delivery-api/pkg/response"

	"github.com/google/uuid"
)

type WebhookIncomingOrderPayload struct {
	OrderID   uuid.UUID `json:"order_id"`
	CreatedAt time.Time `json:"created_at"`
	Comment   *string   `json:"comment,omitempty"`
	Items     []struct {
		MenuItemID uuid.UUID `json:"item_id"`
		Name       string    `json:"name"`
		Quantity   int       `json:"quantity"`
	} `json:"items"`
}

type WebhookHandler struct {
	cfg        *config.Config
	coreClient *client.CoreClient
	stepDelay  time.Duration
}

func NewWebhookHandler(cfg *config.Config, coreClient *client.CoreClient) *WebhookHandler {
	return &WebhookHandler{
		cfg:        cfg,
		coreClient: coreClient,
		stepDelay:  2 * time.Second,
	}
}

func (h *WebhookHandler) SetStepDelay(d time.Duration) {
	h.stepDelay = d
}

func (h *WebhookHandler) HandleIncomingOrder(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Unable to read request body")
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()

	receivedSig := r.Header.Get("X-Signature")
	expectedSig := calculateHMAC(bodyBytes, h.cfg.WebhookSecret)

	if !hmac.Equal([]byte(receivedSig), []byte(expectedSig)) {
		slog.Warn("Rejected incoming webhook due to invalid HMAC signature", "signature", receivedSig)
		response.Error(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "HMAC signature mismatch")
		return
	}

	var payload WebhookIncomingOrderPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid JSON payload")
		return
	}

	eventID := r.Header.Get("X-Event-ID")
	slog.Info("Restaurant POS received new order via webhook",
		"event_id", eventID,
		"order_id", payload.OrderID.String(),
		"items_count", len(payload.Items),
	)

	response.OK(w, map[string]bool{"received": true})

	if h.cfg.AutoAccept {
		go h.SimulateKitchenPipeline(payload.OrderID.String())
	}
}

func (h *WebhookHandler) SimulateKitchenPipeline(orderID string) {
	delay := h.stepDelay
	if delay <= 0 {
		delay = 2 * time.Second
	}

	steps := []struct {
		name   string
		action func(context.Context) error
	}{
		{"AcceptOrder", func(ctx context.Context) error { return h.coreClient.AcceptOrder(ctx, orderID, 25) }},
		{"Cooking", func(ctx context.Context) error { return h.coreClient.UpdateKitchenStatus(ctx, orderID, "COOKING") }},
		{"ReadyForPickup", func(ctx context.Context) error { return h.coreClient.UpdateKitchenStatus(ctx, orderID, "READY_FOR_PICKUP") }},
	}

	for _, step := range steps {
		time.Sleep(delay)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := step.action(ctx)
		cancel()

		if err != nil {
			slog.Error("Kitchen pipeline step failed", "step", step.name, "order_id", orderID, "error", err)
			return
		}
		slog.Info("Kitchen pipeline step completed", "step", step.name, "order_id", orderID)
	}
}

func calculateHMAC(data []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
