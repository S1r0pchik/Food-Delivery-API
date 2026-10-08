package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("Failed to write JSON response", "status", status, "error", err)
		}
	}
}

func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorPayload{
		Code:    code,
		Message: message,
	})
}
