// Package handlers berisi HTTP handler untuk setiap endpoint API.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/dharmayuset/ai-be-sre/backend/internal/utils"
)

// Validator singleton — re-use untuk performa.
var validate = validator.New()

// decodeJSON membaca + validasi body JSON dengan batas size.
// Pakai DisallowUnknownFields supaya request "rapi" (tidak bisa kirim
// field random yang dipakai untuk payload smuggling).
func decodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB max body
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("body kosong")
		}
		return err
	}
	if err := validate.Struct(dst); err != nil {
		return err
	}
	return nil
}

// formatValidationErrors mengubah error validator -> map yang ramah UI.
func formatValidationErrors(err error) any {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		out := make(map[string]string, len(ve))
		for _, fe := range ve {
			out[fe.Field()] = fe.Tag()
		}
		return out
	}
	return err.Error()
}

// writeBadRequest helper khusus invalid body / validation.
func writeBadRequest(w http.ResponseWriter, err error) {
	utils.WriteJSON(w, http.StatusBadRequest, utils.APIError{
		Error:   "request tidak valid",
		Code:    "BAD_REQUEST",
		Details: formatValidationErrors(err),
	})
}
