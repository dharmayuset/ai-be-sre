package middleware

import (
	"net/http"
	"strings"

	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
)

// SecurityHeaders menambahkan header keamanan standar di setiap response.
// Sebagian besar berlaku untuk respons HTML; tetap aman untuk JSON.
func SecurityHeaders(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			// Mencegah MIME-sniffing oleh browser
			h.Set("X-Content-Type-Options", "nosniff")
			// Mencegah halaman di-iframe (clickjacking)
			h.Set("X-Frame-Options", "DENY")
			// Tidak kirim referrer ke origin lain
			h.Set("Referrer-Policy", "no-referrer")
			// Batasi kemampuan API yang berbahaya
			h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			// Default CSP ketat untuk respons API
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			// HSTS hanya kalau production (HTTP -> HTTPS)
			if cfg.IsProduction() {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP mengambil IP client. Hormati X-Forwarded-For kalau ada
// (di belakang reverse proxy / load balancer).
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Format: "client, proxy1, proxy2" — ambil yang pertama
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fallback ke RemoteAddr (sudah include port: "ip:port")
	if i := strings.LastIndex(r.RemoteAddr, ":"); i > 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}
