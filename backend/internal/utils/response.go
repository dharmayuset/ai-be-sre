package utils

import (
	"encoding/json"
	"net/http"
)

// APIError adalah format error standar yang dikirim ke client.
// Dipakai semua handler supaya frontend gampang parsing.
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// WriteJSON menulis response JSON dengan status code.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError shortcut untuk response error.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, APIError{Error: message, Code: code})
}

// WriteValidationError untuk error validasi input.
func WriteValidationError(w http.ResponseWriter, details any) {
	WriteJSON(w, http.StatusBadRequest, APIError{
		Error:   "validation failed",
		Code:    "VALIDATION_ERROR",
		Details: details,
	})
}
