package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dharmayuset/ai-be-sre/backend/internal/audit"
	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	"github.com/dharmayuset/ai-be-sre/backend/internal/email"
	ldapsvc "github.com/dharmayuset/ai-be-sre/backend/internal/ldap"
	"github.com/dharmayuset/ai-be-sre/backend/internal/middleware"
	"github.com/dharmayuset/ai-be-sre/backend/internal/utils"
)

// AdminHandler hanya bisa diakses user dengan role=admin.
type AdminHandler struct {
	cfg    *config.Config
	ldap   *ldapsvc.Client
	mailer email.Sender
	audit  audit.Logger
	logger *slog.Logger
}

func NewAdminHandler(cfg *config.Config, ldap *ldapsvc.Client, mailer email.Sender,
	auditLog audit.Logger, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{cfg: cfg, ldap: ldap, mailer: mailer, audit: auditLog, logger: logger}
}

// ---------- Dashboard Stats ----------

func (h *AdminHandler) DashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.audit.Stats(r.Context(), time.Now().Add(-24*time.Hour))
	if err != nil {
		h.logger.Error("stats failed", slog.Any("err", err))
		utils.WriteError(w, http.StatusInternalServerError, "ERROR", "gagal mengambil statistik")
		return
	}
	utils.WriteJSON(w, http.StatusOK, stats)
}

// ---------- Users List ----------

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := parseIntDefault(r.URL.Query().Get("limit"), 100)

	users, err := h.ldap.ListUsers(q, limit)
	if err != nil {
		h.logger.Error("ldap list users failed", slog.Any("err", err))
		utils.WriteError(w, http.StatusBadGateway, "LDAP_ERROR", "gagal ambil daftar user")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"users": users,
		"count": len(users),
	})
}

// ---------- User Detail ----------

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if username == "" {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "username required")
		return
	}
	user, err := h.ldap.GetUser(username)
	if err != nil {
		if errors.Is(err, ldapsvc.ErrUserNotFound) {
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "user tidak ditemukan")
			return
		}
		utils.WriteError(w, http.StatusBadGateway, "LDAP_ERROR", "gagal ambil user")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{"user": user})
}

// ---------- Admin Reset User Password ----------

func (h *AdminHandler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	username := chi.URLParam(r, "username")
	if username == "" {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "username required")
		return
	}

	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	tempPwd, err := utils.GenerateTemporaryPassword(h.cfg.TempPasswordLength)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "ERROR", "gagal generate password")
		return
	}
	email, err := h.ldap.ResetPasswordAdmin(username, tempPwd)
	if err != nil {
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor: c.Username, Target: username,
			Action: audit.ActionAdminReset, Status: audit.StatusFailure,
			IPAddress: ip, UserAgent: ua,
			Message: classifyPasswordErr(err),
		})
		switch {
		case errors.Is(err, ldapsvc.ErrUserNotFound):
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "user tidak ditemukan")
		case errors.Is(err, ldapsvc.ErrNoEmail):
			utils.WriteError(w, http.StatusBadRequest, "NO_EMAIL", "user tidak punya email terdaftar")
		case errors.Is(err, ldapsvc.ErrPasswordPolicy):
			utils.WriteError(w, http.StatusBadRequest, "POLICY", "tidak lulus password policy")
		default:
			utils.WriteError(w, http.StatusBadGateway, "LDAP_ERROR", "gagal reset password")
		}
		return
	}

	if err := h.mailer.SendTemporaryPassword(email, username, tempPwd); err != nil {
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor: c.Username, Target: username,
			Action: audit.ActionAdminReset, Status: audit.StatusFailure,
			IPAddress: ip, UserAgent: ua,
			Message: "reset OK, email failed: " + err.Error(),
		})
		utils.WriteError(w, http.StatusBadGateway, "EMAIL_FAILED",
			"password sudah di-reset tapi gagal kirim email")
		return
	}

	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor: c.Username, Target: username,
		Action: audit.ActionAdminReset, Status: audit.StatusSuccess,
		IPAddress: ip, UserAgent: ua,
		Message: "sent to " + utils.MaskEmail(email),
	})
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"message":     "password berhasil di-reset, email sudah dikirim ke user",
		"maskedEmail": utils.MaskEmail(email),
	})
}

// ---------- Lock / Unlock User ----------

type lockRequest struct {
	Lock bool `json:"lock"`
}

func (h *AdminHandler) SetUserLock(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	username := chi.URLParam(r, "username")
	if username == "" {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "username required")
		return
	}
	if username == c.Username {
		utils.WriteError(w, http.StatusBadRequest, "SELF_LOCK",
			"tidak bisa lock akun Anda sendiri")
		return
	}
	var req lockRequest
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	if err := h.ldap.SetUserLock(username, req.Lock); err != nil {
		action := audit.ActionLockUser
		if !req.Lock {
			action = audit.ActionUnlockUser
		}
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor: c.Username, Target: username, Action: action,
			Status: audit.StatusFailure, IPAddress: ip, UserAgent: ua,
			Message: err.Error(),
		})
		switch {
		case errors.Is(err, ldapsvc.ErrUserNotFound):
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "user tidak ditemukan")
		case errors.Is(err, ldapsvc.ErrPermissionDenied):
			utils.WriteError(w, http.StatusForbidden, "FORBIDDEN", "service account tidak punya hak")
		default:
			utils.WriteError(w, http.StatusBadGateway, "LDAP_ERROR", "gagal update user")
		}
		return
	}

	action := audit.ActionLockUser
	msg := "user locked"
	if !req.Lock {
		action = audit.ActionUnlockUser
		msg = "user unlocked"
	}
	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor: c.Username, Target: username, Action: action,
		Status: audit.StatusSuccess, IPAddress: ip, UserAgent: ua,
	})
	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": msg})
}

// ---------- Audit Log ----------

func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := audit.ListFilter{
		Actor:  q.Get("actor"),
		Target: q.Get("target"),
		Action: q.Get("action"),
		Status: q.Get("status"),
		Limit:  parseIntDefault(q.Get("limit"), 50),
		Offset: parseIntDefault(q.Get("offset"), 0),
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.From = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.To = t
		}
	}

	entries, total, err := h.audit.List(r.Context(), filter)
	if err != nil {
		h.logger.Error("audit list failed", slog.Any("err", err))
		utils.WriteError(w, http.StatusInternalServerError, "ERROR", "gagal ambil audit log")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"items":  entries,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// ---------- helpers ----------

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
