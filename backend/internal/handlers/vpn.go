package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/dharmayuset/ai-be-sre/backend/internal/audit"
	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	"github.com/dharmayuset/ai-be-sre/backend/internal/email"
	"github.com/dharmayuset/ai-be-sre/backend/internal/middleware"
	"github.com/dharmayuset/ai-be-sre/backend/internal/pritunl"
	"github.com/dharmayuset/ai-be-sre/backend/internal/utils"
)

// VPNHandler menangani endpoint kirim VPN profile.
type VPNHandler struct {
	cfg     *config.Config
	pritunl *pritunl.Client
	mailer  email.Sender
	audit   audit.Logger
	logger  *slog.Logger
}

func NewVPNHandler(cfg *config.Config, pritunlClient *pritunl.Client, mailer email.Sender,
	auditLog audit.Logger, logger *slog.Logger) *VPNHandler {
	return &VPNHandler{cfg: cfg, pritunl: pritunlClient, mailer: mailer, audit: auditLog, logger: logger}
}

// ---------- Send VPN Profile ----------

type sendVPNProfileReq struct {
	Email string `json:"email" validate:"required,email,max=256"`
}

// SendVPNProfile mencari user di Pritunl berdasarkan email, download profile,
// lalu kirim ke email tersebut.
//
// Endpoint ini PUBLIK (tidak butuh login) supaya user yang butuh VPN profile
// bisa self-service. Security:
//   - Rate limit per IP (di-wire di router)
//   - Anti enumeration: selalu return generic message
//   - Audit log setiap request (sukses & gagal)
func (h *VPNHandler) SendVPNProfile(w http.ResponseWriter, r *http.Request) {
	var req sendVPNProfileReq
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err)
		return
	}

	ip := middleware.ClientIP(r)
	ua := r.UserAgent()

	// Generic message (anti enumeration — sama untuk valid/invalid email)
	genericMsg := map[string]string{
		"message": "Jika email terdaftar di VPN system, profile sudah dikirim. Silakan cek inbox (juga folder spam).",
	}

	// 1) Cari user di Pritunl berdasarkan email
	user, err := h.pritunl.FindUserByEmail(req.Email)
	if err != nil {
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     req.Email,
			Target:    req.Email,
			Action:    audit.ActionSendVPNProfile,
			Status:    audit.StatusFailure,
			IPAddress: ip,
			UserAgent: ua,
			Message:   classifyPritunlErr(err),
		})
		// Connection error → return 502. Lainnya → generic 200.
		if errors.Is(err, pritunl.ErrConnection) {
			h.logger.Error("pritunl connection failed", slog.Any("err", err))
			utils.WriteError(w, http.StatusBadGateway, "VPN_UNAVAILABLE",
				"server VPN tidak tersedia, coba lagi nanti")
			return
		}
		// User not found / no org → tetap return generic (anti enumeration)
		utils.WriteJSON(w, http.StatusOK, genericMsg)
		return
	}

	// 2) Download VPN profile (.tar)
	profileData, filename, err := h.pritunl.DownloadProfile(user.OrgID, user.ID)
	if err != nil {
		h.logger.Error("pritunl download profile failed",
			slog.String("user", user.Name),
			slog.Any("err", err))
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     req.Email,
			Target:    user.Name,
			Action:    audit.ActionSendVPNProfile,
			Status:    audit.StatusFailure,
			IPAddress: ip,
			UserAgent: ua,
			Message:   "download failed: " + err.Error(),
		})
		utils.WriteError(w, http.StatusBadGateway, "PROFILE_DOWNLOAD_FAILED",
			"gagal mengambil VPN profile, coba lagi nanti")
		return
	}

	// 3) Kirim email dengan attachment
	if err := h.mailer.SendVPNProfile(req.Email, user.Name, profileData, filename); err != nil {
		h.logger.Error("send vpn profile email failed",
			slog.String("user", user.Name),
			slog.Any("err", err))
		_ = h.audit.Log(r.Context(), audit.Entry{
			Actor:     req.Email,
			Target:    user.Name,
			Action:    audit.ActionSendVPNProfile,
			Status:    audit.StatusFailure,
			IPAddress: ip,
			UserAgent: ua,
			Message:   "email send failed: " + err.Error(),
		})
		utils.WriteError(w, http.StatusBadGateway, "EMAIL_FAILED",
			"VPN profile berhasil diambil tapi gagal kirim email, coba lagi nanti")
		return
	}

	// 4) Success
	_ = h.audit.Log(r.Context(), audit.Entry{
		Actor:     req.Email,
		Target:    user.Name,
		Action:    audit.ActionSendVPNProfile,
		Status:    audit.StatusSuccess,
		IPAddress: ip,
		UserAgent: ua,
		Message:   "profile sent to " + utils.MaskEmail(req.Email),
	})

	utils.WriteJSON(w, http.StatusOK, genericMsg)
}

func classifyPritunlErr(err error) string {
	switch {
	case errors.Is(err, pritunl.ErrUserNotFound):
		return "user not found in pritunl"
	case errors.Is(err, pritunl.ErrNoOrganization):
		return "no organization configured"
	case errors.Is(err, pritunl.ErrConnection):
		return "pritunl connection failed"
	case errors.Is(err, pritunl.ErrProfileDownload):
		return "profile download failed"
	default:
		return "pritunl error: " + err.Error()
	}
}
