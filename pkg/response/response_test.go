package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"food-delivery-api/pkg/response"
)

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"message": "ok"}

	response.JSON(rec, http.StatusCreated, data)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("unexpected content type: %s", ct)
	}

	var res map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res["message"] != "ok" {
		t.Errorf("payload mismatch: %v", res)
	}
}

func TestError(t *testing.T) {
	rec := httptest.NewRecorder()
	response.Error(rec, http.StatusBadRequest, "INVALID_INPUT", "Field is required")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	var payload response.ErrorPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	if payload.Code != "INVALID_INPUT" || payload.Message != "Field is required" {
		t.Errorf("payload mismatch: %+v", payload)
	}
}
