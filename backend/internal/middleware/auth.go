package middleware

import (
	"net/http"

	"github.com/dharmayuset/ai-be-sre/backend/internal/auth"
	"github.com/dharmayuset/ai-be-sre/backend/internal/models"
	"github.com/dharmayuset/ai-be-sre/backend/internal/utils"
)

// RequireAuth memastikan request punya access token yang valid.
// Claims di-attach ke context untuk dipakai handler.
func RequireAuth(jwtMgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := auth.GetTokenFromRequest(r)
			if tok == "" {
				utils.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
				return
			}
			claims, err := jwtMgr.VerifyAccess(tok)
			if err != nil {
				utils.WriteError(w, http.StatusUnauthorized, "INVALID_TOKEN", "token tidak valid atau expired")
				return
			}
			next.ServeHTTP(w, WithClaims(r, claims))
		})
	}
}

// RequireRole memastikan claims punya role yang dibutuhkan.
// Pasang setelah RequireAuth.
func RequireRole(role models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := ClaimsFromContext(r.Context())
			if c == nil {
				utils.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
				return
			}
			if c.Role != string(role) {
				utils.WriteError(w, http.StatusForbidden, "FORBIDDEN", "akses ditolak: butuh role "+string(role))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
