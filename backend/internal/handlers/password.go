package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/dharmayuset/ai-be-sre/backend/internal/audit"
	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	"github.com/dharmayuset/ai-be-sre/backend/internal/email"
	ldapsvc "github.com/dharmayuset/ai-be-sre/backend/internal/ldap"
	"github.com/dharmayuset/ai-be-sre/backend/internal/middleware"
	"github.com/dharmayuset/ai-be-sre/backend/internal/utils"
)

// PasswordHandler menangani change & reset password.
type PasswordHandler struct {
	cfg    *config.Config
	ldap   *ldapsvc.Client
	mailer email.Sender
	audit  audit.Logger
	logger *slog.Logger
}

func NewPasswordHandler(cfg *config.Config, ldap *ldapsvc.Client, mailer email.Sender,
	auditLog audit.Logger, logger *slog.Logger) *PasswordHandler {
	return &PasswordHandler{cfg: cfg, ldap: ldap, mailer: mailer, audit: auditLog, logger: logger}
}

// ---------- Change Password (user yang login mengganti sendiri) ----------

type changePasswordReq struct {
	OldPassword string `json:"oldPassword" validate:"required,min=1,max=256"`
	NewPassword string `json:"newPassword" validate:"required,min=12,max=256"`
}

func (h *PasswordHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	c := middleware.ClaimsFromContext(r.Context())
	if c == nil {
		utils.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "auth required")
		return
	}
	var req changePasswordReq
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err)
		return
	}
	if req.OldPassword == req.NewPassword {
		utils.WriteError(w, http.StatusBadRequest, "SAME_PASSWORD",
			"password baru harus berbeda dari password lama")
		return
	}

	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	if err := h.ldap.ChangePasswordSelf(c.Username, req.OldPassword, req.NewPassword); err != nil {
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     c.Username,
			Target:    c.Username,
			Action:    audit.ActionChangePassword,
			Status:    audit.StatusFailure,
			IPAddress: ip,
			UserAgent: ua,
			Message:   classifyPasswordErr(err),
		})
		switch {
		case errors.Is(err, ldapsvc.ErrInvalidCredentials):
			utils.WriteError(w, http.StatusUnauthorized, "INVALID_OLD_PASSWORD",
				"password lama salah")
		case errors.Is(err, ldapsvc.ErrPasswordPolicy):
			utils.WriteError(w, http.StatusBadRequest, "WEAK_PASSWORD",
				"password tidak memenuhi kebijakan keamanan FreeIPA")
		case errors.Is(err, ldapsvc.ErrAccountLocked):
			utils.WriteError(w, http.StatusForbidden, "ACCOUNT_LOCKED", "akun terkunci")
		case errors.Is(err, ldapsvc.ErrConnection):
			utils.WriteError(w, http.StatusBadGateway, "LDAP_UNAVAILABLE", "ldap tidak tersedia")
		default:
			h.logger.Error("change password failed", slog.Any("err", err))
			utils.WriteError(w, http.StatusInternalServerError, "ERROR", "gagal mengubah password")
		}
		return
	}

	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor:     c.Username,
		Target:    c.Username,
		Action:    audit.ActionChangePassword,
		Status:    audit.StatusSuccess,
		IPAddress: ip,
		UserAgent: ua,
	})

	// Notifikasi (best-effort, jangan gagal-kan request)
	if c.Email != "" {
		go func(to, username string) {
			if err := h.mailer.SendPasswordChanged(to, username); err != nil {
				h.logger.Warn("send notification failed", slog.Any("err", err))
			}
		}(c.Email, c.Username)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "password berhasil diubah",
	})
}

// ---------- Reset Password (kirim temp password ke email user) ----------
//
// Endpoint ini PUBLIK (tidak butuh login) supaya user yang lupa password
// tetap bisa pakai. Untuk security:
//   - Rate limit per IP (di-wire di router)
//   - Anti user enumeration: selalu return generic message
//   - Audit failure tetap dicatat untuk monitoring abuse

type resetRequest struct {
	Username string `json:"username" validate:"required,min=1,max=64"`
}

type resetResponse struct {
	// Generic message - sama untuk valid/invalid username
	Message string `json:"message"`
}

func (h *PasswordHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err)
		return
	}

	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	// Generic message yang selalu return (anti user enumeration).
	genericMsg := resetResponse{
		Message: "Jika username terdaftar dan punya email, instruksi reset password sudah dikirim.",
	}

	tempPwd, err := utils.GenerateTemporaryPassword(h.cfg.TempPasswordLength)
	if err != nil {
		h.logger.Error("generate temp password failed", slog.Any("err", err))
		utils.WriteError(w, http.StatusInternalServerError, "ERROR", "gagal generate password")
		return
	}

	email, err := h.ldap.ResetPasswordAdmin(req.Username, tempPwd)
	if err != nil {
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     req.Username,
			Target:    req.Username,
			Action:    audit.ActionResetPassword,
			Status:    audit.StatusFailure,
			IPAddress: ip,
			UserAgent: ua,
			Message:   classifyPasswordErr(err),
		})
		// Tetap return generic 200 supaya tidak leak info user enumeration.
		// Connection / system error tetap return 502 supaya monitoring tahu.
		if errors.Is(err, ldapsvc.ErrConnection) {
			utils.WriteError(w, http.StatusBadGateway, "LDAP_UNAVAILABLE", "ldap tidak tersedia")
			return
		}
		utils.WriteJSON(w, http.StatusOK, genericMsg)
		return
	}

	if err := h.mailer.SendTemporaryPassword(email, req.Username, tempPwd); err != nil {
		// Password sudah ter-reset di LDAP. Catat error untuk follow-up admin.
		h.logger.Error("send email failed",
			slog.String("user", req.Username),
			slog.Any("err", err))
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     req.Username,
			Target:    req.Username,
			Action:    audit.ActionResetPassword,
			Status:    audit.StatusFailure,
			IPAddress: ip,
			UserAgent: ua,
			Message:   "password reset OK, email send failed: " + err.Error(),
		})
		utils.WriteError(w, http.StatusBadGateway, "EMAIL_FAILED",
			"password sudah di-reset tapi gagal kirim email; hubungi admin")
		return
	}

	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor:     req.Username,
		Target:    req.Username,
		Action:    audit.ActionResetPassword,
		Status:    audit.StatusSuccess,
		IPAddress: ip,
		UserAgent: ua,
		Message:   "sent to " + utils.MaskEmail(email),
	})

	utils.WriteJSON(w, http.StatusOK, genericMsg)
}

func classifyPasswordErr(err error) string {
	switch {
	case errors.Is(err, ldapsvc.ErrInvalidCredentials):
		return "invalid old password"
	case errors.Is(err, ldapsvc.ErrPasswordPolicy):
		return "password policy violation"
	case errors.Is(err, ldapsvc.ErrAccountLocked):
		return "account locked"
	case errors.Is(err, ldapsvc.ErrUserNotFound):
		return "user not found"
	case errors.Is(err, ldapsvc.ErrNoEmail):
		return "no email registered"
	case errors.Is(err, ldapsvc.ErrConnection):
		return "ldap unavailable"
	case errors.Is(err, ldapsvc.ErrPermissionDenied):
		return "permission denied"
	default:
		return "error: " + err.Error()
	}
}
