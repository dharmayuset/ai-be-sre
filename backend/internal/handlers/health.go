package handlers

import (
	"net/http"

	"github.com/dharmayuset/ai-be-sre/backend/internal/utils"
)

// Health adalah handler liveness sederhana.
// Untuk readiness (cek LDAP/DB), bisa di-extend nanti.
func Health(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
