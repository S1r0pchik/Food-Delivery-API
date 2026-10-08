package client

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"food-delivery-api/internal/core/domain"

	"github.com/google/uuid"
)

type WebhookItemPayload struct {
	MenuItemID uuid.UUID `json:"item_id"`
	Name       string    `json:"name"`
	Quantity   int       `json:"quantity"`
}

type WebhookOrderCreatedPayload struct {
	OrderID   uuid.UUID            `json:"order_id"`
	CreatedAt time.Time            `json:"created_at"`
	Comment   *string              `json:"comment,omitempty"`
	Items     []WebhookItemPayload `json:"items"`
}

type RestaurantWebhookClient struct {
	httpClient *http.Client
}

func NewRestaurantWebhookClient() *RestaurantWebhookClient {
	return &RestaurantWebhookClient{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *RestaurantWebhookClient) SendOrderCreated(ctx context.Context, restaurant *domain.Restaurant, order *domain.Order) error {
	itemsPayload := make([]WebhookItemPayload, len(order.Items))
	for i, it := range order.Items {
		var itemID uuid.UUID
		if it.MenuItemID != nil {
			itemID = *it.MenuItemID
		}
		itemsPayload[i] = WebhookItemPayload{
			MenuItemID: itemID,
			Name:       it.NameAtOrder,
			Quantity:   it.Quantity,
		}
	}

	payload := WebhookOrderCreatedPayload{
		OrderID:   order.ID,
		CreatedAt: order.CreatedAt,
		Comment:   order.Comment,
		Items:     itemsPayload,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, restaurant.WebhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	eventID := uuid.New().String()
	signature := calculateHMAC(bodyBytes, restaurant.WebhookSecret)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-ID", eventID)
	req.Header.Set("X-Signature", signature)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook delivery failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	_, readErr := io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook target returned non-2xx status: %d", resp.StatusCode)
	}
	if readErr != nil {
		return fmt.Errorf("failed to read webhook response: %w", readErr)
	}

	return nil
}

func calculateHMAC(data []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
