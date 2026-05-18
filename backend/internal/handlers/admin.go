package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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

// ---------- Users List (with filter) ----------

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status") // "active", "inactive", "" (all)
	limit := parseIntDefault(r.URL.Query().Get("limit"), 500)

	users, err := h.ldap.ListUsersFiltered(q, status, limit)
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

// ---------- User Stats ----------

// UserStats mengembalikan jumlah user aktif & tidak aktif untuk dashboard.
func (h *AdminHandler) UserStats(w http.ResponseWriter, r *http.Request) {
	active, inactive, total, err := h.ldap.UserStats()
	if err != nil {
		h.logger.Error("user stats failed", slog.Any("err", err))
		utils.WriteError(w, http.StatusBadGateway, "LDAP_ERROR", "gagal ambil statistik user")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"active":   active,
		"inactive": inactive,
		"total":    total,
	})
}

// ---------- Delete User ----------

type deleteUserReq struct {
	Username string `json:"username" validate:"required,min=1,max=64"`
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	username := chi.URLParam(r, "username")
	if username == "" {
		utils.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "username required")
		return
	}
	if username == c.Username {
		utils.WriteError(w, http.StatusBadRequest, "SELF_DELETE",
			"tidak bisa menghapus akun Anda sendiri")
		return
	}

	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	if err := h.ldap.DeleteUser(username); err != nil {
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor: c.Username, Target: username,
			Action: audit.ActionDeleteUser, Status: audit.StatusFailure,
			IPAddress: ip, UserAgent: ua,
			Message: err.Error(),
		})
		switch {
		case errors.Is(err, ldapsvc.ErrUserNotFound):
			utils.WriteError(w, http.StatusNotFound, "NOT_FOUND", "user tidak ditemukan")
		case errors.Is(err, ldapsvc.ErrPermissionDenied):
			utils.WriteError(w, http.StatusForbidden, "FORBIDDEN",
				"service account tidak punya hak hapus user")
		default:
			utils.WriteError(w, http.StatusBadGateway, "LDAP_ERROR", "gagal menghapus user")
		}
		return
	}

	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor: c.Username, Target: username,
		Action: audit.ActionDeleteUser, Status: audit.StatusSuccess,
		IPAddress: ip, UserAgent: ua,
	})
	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "user " + username + " berhasil dihapus",
	})
}

// ---------- Batch Delete Users ----------

type batchDeleteReq struct {
	Usernames []string `json:"usernames" validate:"required,min=1,max=100,dive,min=1,max=64"`
}

type batchDeleteResult struct {
	Username string `json:"username"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

func (h *AdminHandler) BatchDeleteUsers(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	var req batchDeleteReq
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err)
		return
	}

	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	// Cegah admin hapus dirinya sendiri
	for _, u := range req.Usernames {
		if u == c.Username {
			utils.WriteError(w, http.StatusBadRequest, "SELF_DELETE",
				"tidak bisa menghapus akun Anda sendiri (username: "+u+")")
			return
		}
	}

	results := h.ldap.DeleteUsers(req.Usernames)

	var successCount, failCount int
	output := make([]batchDeleteResult, 0, len(req.Usernames))

	for _, username := range req.Usernames {
		err := results[username]
		if err == nil {
			successCount++
			output = append(output, batchDeleteResult{Username: username, Success: true})
			_ = h.audit.Log(r.Context(), audit.Entry{
				Actor: c.Username, Target: username,
				Action: audit.ActionBatchDelete, Status: audit.StatusSuccess,
				IPAddress: ip, UserAgent: ua,
			})
		} else {
			failCount++
			output = append(output, batchDeleteResult{
				Username: username, Success: false, Error: err.Error(),
			})
			_ = h.audit.Log(r.Context(), audit.Entry{
				Actor: c.Username, Target: username,
				Action: audit.ActionBatchDelete, Status: audit.StatusFailure,
				IPAddress: ip, UserAgent: ua,
				Message: err.Error(),
			})
		}
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"results":      output,
		"successCount": successCount,
		"failCount":    failCount,
		"total":        len(req.Usernames),
	})
}

// ---------- Export Users CSV ----------

// ExportUsersCSV menghasilkan file CSV dari daftar user yang ter-filter.
// Client download langsung (Content-Disposition: attachment).
func (h *AdminHandler) ExportUsersCSV(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")

	users, err := h.ldap.ListUsersFiltered(q, status, 1000)
	if err != nil {
		h.logger.Error("ldap list users for CSV failed", slog.Any("err", err))
		utils.WriteError(w, http.StatusBadGateway, "LDAP_ERROR", "gagal ambil daftar user")
		return
	}

	// Set headers supaya browser download sebagai file
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"users_export.csv\"")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	// BOM untuk Excel agar baca UTF-8 benar
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	// CSV Header
	_, _ = w.Write([]byte("Username,Display Name,First Name,Last Name,Email,Status,Groups\r\n"))

	for _, u := range users {
		statusStr := "active"
		if u.Locked {
			statusStr = "locked"
		}
		groups := ""
		if len(u.Groups) > 0 {
			groups = strings.Join(u.Groups, "; ")
		}
		// Escape CSV fields (double-quote fields that may contain comma/quote/newline)
		line := csvEscape(u.Username) + "," +
			csvEscape(u.DisplayName) + "," +
			csvEscape(u.FirstName) + "," +
			csvEscape(u.LastName) + "," +
			csvEscape(u.Email) + "," +
			csvEscape(statusStr) + "," +
			csvEscape(groups) + "\r\n"
		_, _ = w.Write([]byte(line))
	}
}

// csvEscape wraps a field in double-quotes if it contains special characters.
func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\r\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
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
