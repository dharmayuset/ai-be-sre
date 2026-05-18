package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dharmayuset/ai-be-sre/backend/internal/audit"
	"github.com/dharmayuset/ai-be-sre/backend/internal/auth"
	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	ldapsvc "github.com/dharmayuset/ai-be-sre/backend/internal/ldap"
	"github.com/dharmayuset/ai-be-sre/backend/internal/middleware"
	"github.com/dharmayuset/ai-be-sre/backend/internal/models"
	"github.com/dharmayuset/ai-be-sre/backend/internal/utils"
)

// AuthHandler menangani login/logout/refresh/me.
type AuthHandler struct {
	cfg    *config.Config
	jwt    *auth.Manager
	ldap   *ldapsvc.Client
	audit  audit.Logger
	logger *slog.Logger
}

func NewAuthHandler(cfg *config.Config, jwtMgr *auth.Manager, ldap *ldapsvc.Client,
	auditLog audit.Logger, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{cfg: cfg, jwt: jwtMgr, ldap: ldap, audit: auditLog, logger: logger}
}

// ---------- Login ----------

type loginRequest struct {
	Username string `json:"username" validate:"required,min=1,max=64"`
	Password string `json:"password" validate:"required,min=1,max=256"`
}

type loginResponse struct {
	User *models.User `json:"user"`
}

// Login melakukan bind LDAP. Sukses -> set HttpOnly cookie + return user info.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err)
		return
	}

	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	user, err := h.ldap.Authenticate(req.Username, req.Password)
	if err != nil {
		// Audit failed attempt
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     req.Username,
			Action:    audit.ActionLoginFailed,
			Status:    audit.StatusFailure,
			IPAddress: ip,
			UserAgent: ua,
			Message:   classifyAuthErr(err),
		})

		// Generic error message untuk mencegah user enumeration.
		switch {
		case errors.Is(err, ldapsvc.ErrAccountLocked):
			utils.WriteError(w, http.StatusForbidden, "ACCOUNT_LOCKED",
				"akun Anda terkunci, hubungi admin")
		case errors.Is(err, ldapsvc.ErrConnection):
			utils.WriteError(w, http.StatusBadGateway, "LDAP_UNAVAILABLE",
				"server otentikasi tidak tersedia")
		default:
			utils.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS",
				"username atau password salah")
		}
		return
	}

	access, refresh, err := h.jwt.IssueTokenPair(user)
	if err != nil {
		h.logger.Error("issue token failed", slog.Any("err", err))
		utils.WriteError(w, http.StatusInternalServerError, "TOKEN_ERROR", "gagal membuat token")
		return
	}
	auth.SetAuthCookies(w, h.cfg, access, refresh)

	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor:     user.Username,
		Action:    audit.ActionLogin,
		Status:    audit.StatusSuccess,
		IPAddress: ip,
		UserAgent: ua,
	})
	utils.WriteJSON(w, http.StatusOK, loginResponse{User: user})
}

// ---------- Logout ----------

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearAuthCookies(w, h.cfg)

	if c := middleware.ClaimsFromContext(r.Context()); c != nil {
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     c.Username,
			Action:    audit.ActionLogout,
			Status:    audit.StatusSuccess,
			IPAddress: middleware.ClientIP(r),
			UserAgent: r.UserAgent(),
		})
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// ---------- Refresh ----------

// Refresh membuat access token baru pakai refresh token di cookie.
// Refresh token tetap dipakai sampai expired (rotation tidak diterapkan
// untuk simplicity; bisa di-extend dengan token blacklist di DB).
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	tok := auth.GetRefreshTokenFromRequest(r)
	if tok == "" {
		utils.WriteError(w, http.StatusUnauthorized, "NO_REFRESH", "refresh token tidak ada")
		return
	}
	claims, err := h.jwt.VerifyRefresh(tok)
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "INVALID_REFRESH", "refresh token tidak valid")
		return
	}

	// Re-fetch user dari LDAP supaya groups/role selalu fresh
	// (mis. user di-remove dari grup admin -> langsung kehilangan akses).
	user, err := h.ldap.GetUser(claims.Username)
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "USER_GONE", "user tidak ditemukan di direktori")
		return
	}
	if user.Locked {
		auth.ClearAuthCookies(w, h.cfg)
		utils.WriteError(w, http.StatusForbidden, "ACCOUNT_LOCKED", "akun terkunci")
		return
	}

	access, err := h.jwt.IssueAccess(user)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "TOKEN_ERROR", "gagal membuat token")
		return
	}
	auth.SetAuthCookies(w, h.cfg, access, tok) // refresh tetap sama
	utils.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

// ---------- Me ----------

// Me mengembalikan info user yang sedang login.
// Selalu re-fetch dari LDAP biar role/group selalu up-to-date.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		utils.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "auth required")
		return
	}
	user, err := h.ldap.GetUser(c.Username)
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "user tidak ditemukan")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

// classifyAuthErr -> string ringkas untuk audit (jangan bocorkan password!)
func classifyAuthErr(err error) string {
	switch {
	case errors.Is(err, ldapsvc.ErrInvalidCredentials):
		return "invalid credentials"
	case errors.Is(err, ldapsvc.ErrAccountLocked):
		return "account locked"
	case errors.Is(err, ldapsvc.ErrConnection):
		return "ldap unavailable"
	case errors.Is(err, ldapsvc.ErrUserNotFound):
		return "user not found"
	default:
		return "auth failed"
	}
}

// ensure ctx import dipakai (kalau handler butuh)
var _ = context.Background
