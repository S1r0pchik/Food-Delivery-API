package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type CoreClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewCoreClient(baseURL, apiKey string) *CoreClient {
	return &CoreClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *CoreClient) AcceptOrder(ctx context.Context, orderID string, cookingTimeMinutes int) error {
	url := fmt.Sprintf("%s/api/v1/partner/orders/%s/accept", c.baseURL, orderID)
	body, err := json.Marshal(map[string]any{"cooking_time_minutes": cookingTimeMinutes})
	if err != nil {
		return fmt.Errorf("failed to marshal accept order request: %w", err)
	}
	return c.sendRequest(ctx, http.MethodPost, url, body)
}

func (c *CoreClient) RejectOrder(ctx context.Context, orderID, reason string) error {
	url := fmt.Sprintf("%s/api/v1/partner/orders/%s/reject", c.baseURL, orderID)
	body, err := json.Marshal(map[string]any{"reason": reason})
	if err != nil {
		return fmt.Errorf("failed to marshal reject order request: %w", err)
	}
	return c.sendRequest(ctx, http.MethodPost, url, body)
}

func (c *CoreClient) UpdateKitchenStatus(ctx context.Context, orderID, status string) error {
	url := fmt.Sprintf("%s/api/v1/partner/orders/%s/status", c.baseURL, orderID)
	body, err := json.Marshal(map[string]any{"status": status})
	if err != nil {
		return fmt.Errorf("failed to marshal kitchen status request: %w", err)
	}
	return c.sendRequest(ctx, http.MethodPatch, url, body)
}

func (c *CoreClient) UpdateItemAvailability(ctx context.Context, itemID string, isAvailable bool) error {
	url := fmt.Sprintf("%s/api/v1/partner/menu/items/%s", c.baseURL, itemID)
	body, err := json.Marshal(map[string]any{"is_available": isAvailable})
	if err != nil {
		return fmt.Errorf("failed to marshal item availability request: %w", err)
	}
	return c.sendRequest(ctx, http.MethodPatch, url, body)
}

func (c *CoreClient) sendRequest(ctx context.Context, method, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to core failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	_, readErr := io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("core api returned status %d", resp.StatusCode)
	}
	if readErr != nil {
		return fmt.Errorf("failed to read core response: %w", readErr)
	}

	return nil
}
