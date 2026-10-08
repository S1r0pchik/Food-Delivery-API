package handler_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"food-delivery-api/internal/restaurant/client"
	"food-delivery-api/internal/restaurant/config"
	"food-delivery-api/internal/restaurant/handler"
)

func TestWebhookHandler_HandleIncomingOrder_SignatureVerification(t *testing.T) {
	secret := "test-hmac-secret"
	cfg := &config.Config{
		WebhookSecret: secret,
		AutoAccept:    false,
	}
	h := handler.NewWebhookHandler(cfg, client.NewCoreClient("http://localhost", "key"))

	payload := `{"order_id":"11111111-1111-1111-1111-111111111111","items":[]}`

	reqInvalid := httptest.NewRequest(http.MethodPost, "/webhooks/orders/incoming", bytes.NewBufferString(payload))
	reqInvalid.Header.Set("X-Signature", "wrong-signature")
	recInvalid := httptest.NewRecorder()

	h.HandleIncomingOrder(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid signature, got %d", recInvalid.Code)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	validSig := hex.EncodeToString(mac.Sum(nil))

	reqValid := httptest.NewRequest(http.MethodPost, "/webhooks/orders/incoming", bytes.NewBufferString(payload))
	reqValid.Header.Set("X-Signature", validSig)
	reqValid.Header.Set("X-Event-ID", "evt-123")
	recValid := httptest.NewRecorder()

	h.HandleIncomingOrder(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Errorf("expected 200 for valid signature, got %d", recValid.Code)
	}
}

func TestWebhookHandler_SimulateKitchenPipeline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config.Config{
		WebhookSecret: "secret",
		AutoAccept:    true,
	}
	coreClient := client.NewCoreClient(srv.URL, "test-api-key")
	h := handler.NewWebhookHandler(cfg, coreClient)
	h.SetStepDelay(1 * time.Millisecond)

	h.SimulateKitchenPipeline("11111111-1111-1111-1111-111111111111")
}
