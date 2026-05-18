// Package middleware berisi HTTP middleware reusable.
package middleware

import (
	"context"
	"net/http"

	"github.com/dharmayuset/ai-be-sre/backend/internal/auth"
)

// ctxKey adalah tipe private supaya tidak collide dengan key dari package lain.
type ctxKey int

const (
	ctxKeyClaims ctxKey = iota
)

// WithClaims menempatkan claims ke context request.
func WithClaims(r *http.Request, c *auth.Claims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKeyClaims, c))
}

// ClaimsFromContext mengambil claims yang sudah di-set middleware Auth.
// Return nil kalau tidak ada (request unauth).
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	v := ctx.Value(ctxKeyClaims)
	if v == nil {
		return nil
	}
	if c, ok := v.(*auth.Claims); ok {
		return c
	}
	return nil
}
