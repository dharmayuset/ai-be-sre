package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/dharmayuset/ai-be-sre/backend/internal/auth"
	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	"github.com/dharmayuset/ai-be-sre/backend/internal/handlers"
	"github.com/dharmayuset/ai-be-sre/backend/internal/middleware"
	"github.com/dharmayuset/ai-be-sre/backend/internal/models"
)

// buildRouter merangkai semua middleware + route ke chi.Router.
//
// Layered protection:
//  1. Recoverer  -> panic-safe
//  2. RealIP / RequestLogger
//  3. Security headers
//  4. CORS
//  5. Rate limit (per endpoint group)
//  6. Auth & role guards (per group)
func buildRouter(cfg *config.Config, logger *slog.Logger,
	authH *handlers.AuthHandler, pwdH *handlers.PasswordHandler, adminH *handlers.AdminHandler,
	jwtMgr *auth.Manager) http.Handler {

	r := chi.NewRouter()

	// Global middleware (applies to all)
	r.Use(middleware.Recoverer(logger))
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.SecurityHeaders(cfg))
	r.Use(chimw.Timeout(30 * time.Second))

	// CORS (frontend Next.js)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{},
		AllowCredentials: true, // perlu karena pakai cookie
		MaxAge:           300,
	}))

	// Healthcheck (no auth, no rate-limit)
	r.Get("/health", handlers.Health)

	// API root
	r.Route("/api/v1", func(r chi.Router) {

		// ---------- Public endpoints (with rate limit) ----------
		r.Group(func(r chi.Router) {
			// Login: limit per IP supaya susah brute-force
			r.With(httprate.LimitByIP(cfg.RateLimitLoginPM, time.Minute)).
				Post("/auth/login", authH.Login)

			r.With(httprate.LimitByIP(cfg.RateLimitAPIPM, time.Minute)).
				Post("/auth/refresh", authH.Refresh)

			// Self-service reset password (publik, lupa password)
			r.With(httprate.LimitByIP(cfg.RateLimitResetPM, time.Minute)).
				Post("/password/reset-request", pwdH.ResetPassword)
		})

		// ---------- Authenticated endpoints ----------
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtMgr))
			r.Use(httprate.LimitByIP(cfg.RateLimitAPIPM, time.Minute))

			r.Post("/auth/logout", authH.Logout)
			r.Get("/auth/me", authH.Me)

			// Change password (user ganti sendiri)
			r.Post("/password/change", pwdH.ChangePassword)
		})

		// ---------- Admin-only endpoints ----------
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(jwtMgr))
			r.Use(middleware.RequireRole(models.RoleAdmin))
			r.Use(httprate.LimitByIP(cfg.RateLimitAPIPM, time.Minute))

			r.Get("/admin/stats", adminH.DashboardStats)
			r.Get("/admin/users", adminH.ListUsers)
			r.Get("/admin/users/{username}", adminH.GetUser)
			r.Post("/admin/users/{username}/reset-password", adminH.ResetUserPassword)
			r.Post("/admin/users/{username}/lock", adminH.SetUserLock)
			r.Get("/admin/audit", adminH.ListAuditLogs)
		})
	})

	return r
}
