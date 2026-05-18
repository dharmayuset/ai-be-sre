package auth

import (
	"net/http"
	"time"

	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
)

// Cookie name constants.
const (
	CookieAccessToken  = "ai_be_sre_access"
	CookieRefreshToken = "ai_be_sre_refresh"
)

// SetAuthCookies set access & refresh cookies dengan flag keamanan
// yang sesuai. HttpOnly + Secure + SameSite=Lax mencegah XSS & CSRF
// trivial.
func SetAuthCookies(w http.ResponseWriter, cfg *config.Config, access, refresh string) {
	secure := cfg.IsProduction()

	http.SetCookie(w, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(cfg.JWTAccessTTL),
		MaxAge:   int(cfg.JWTAccessTTL.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(cfg.JWTRefreshTTL),
		MaxAge:   int(cfg.JWTRefreshTTL.Seconds()),
	})
}

// ClearAuthCookies hapus auth cookies (dipakai saat logout).
func ClearAuthCookies(w http.ResponseWriter, cfg *config.Config) {
	secure := cfg.IsProduction()
	for _, name := range []string{CookieAccessToken, CookieRefreshToken} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}
}

// GetTokenFromRequest mencari access token di:
//  1. Cookie (priority)
//  2. Authorization: Bearer <token> header
func GetTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(CookieAccessToken); err == nil && c.Value != "" {
		return c.Value
	}
	const prefix = "Bearer "
	if h := r.Header.Get("Authorization"); len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}

// GetRefreshTokenFromRequest mencari refresh token di cookie.
func GetRefreshTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(CookieRefreshToken); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}
