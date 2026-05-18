// Package auth menangani JWT issuance & verification.
//
// Strategi:
//   - Access token: short-lived (15 min default), berisi claims user.
//   - Refresh token: longer-lived (8 jam default), opaque-ish JWT yang
//     hanya bisa generate access token baru.
//   - HMAC-SHA256 (HS256). Kalau butuh asymmetric (RS256), tinggal ganti
//     key & method di sini.
//
// Token disimpan di frontend sebagai HttpOnly cookie (set di handler).
// JTI (JWT ID) diisi UUID untuk traceability di audit log.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	"github.com/dharmayuset/ai-be-sre/backend/internal/models"
)

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// Claims yang kita simpan di JWT.
type Claims struct {
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	DisplayName string   `json:"displayName"`
	Role        string   `json:"role"`
	Groups      []string `json:"groups"`
	TokenType   string   `json:"typ"`
	jwt.RegisteredClaims
}

// IsAdmin shortcut.
func (c *Claims) IsAdmin() bool {
	return c.Role == string(models.RoleAdmin)
}

// Manager menerbitkan & memverifikasi token.
type Manager struct {
	cfg *config.Config
	key []byte
}

// NewManager membuat JWT manager dengan secret dari config.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg, key: []byte(cfg.JWTSecret)}
}

// IssueTokenPair membuat access + refresh token untuk user yang sudah
// terverifikasi (mis. setelah berhasil login LDAP).
func (m *Manager) IssueTokenPair(u *models.User) (access, refresh string, err error) {
	access, err = m.sign(u, tokenTypeAccess, m.cfg.JWTAccessTTL)
	if err != nil {
		return "", "", err
	}
	refresh, err = m.sign(u, tokenTypeRefresh, m.cfg.JWTRefreshTTL)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// IssueAccess membuat access token baru (dipakai saat refresh).
func (m *Manager) IssueAccess(u *models.User) (string, error) {
	return m.sign(u, tokenTypeAccess, m.cfg.JWTAccessTTL)
}

func (m *Manager) sign(u *models.User, typ string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Username:    u.Username,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        string(u.Role()),
		Groups:      u.Groups,
		TokenType:   typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.JWTIssuer,
			Subject:   u.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.NewString(),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(m.key)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// VerifyAccess parse & validate access token.
func (m *Manager) VerifyAccess(tokenStr string) (*Claims, error) {
	c, err := m.verify(tokenStr)
	if err != nil {
		return nil, err
	}
	if c.TokenType != tokenTypeAccess {
		return nil, errors.New("token bukan access token")
	}
	return c, nil
}

// VerifyRefresh parse & validate refresh token.
func (m *Manager) VerifyRefresh(tokenStr string) (*Claims, error) {
	c, err := m.verify(tokenStr)
	if err != nil {
		return nil, err
	}
	if c.TokenType != tokenTypeRefresh {
		return nil, errors.New("token bukan refresh token")
	}
	return c, nil
}

func (m *Manager) verify(tokenStr string) (*Claims, error) {
	parser := jwt.NewParser(
		jwt.WithIssuer(m.cfg.JWTIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	t, err := parser.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		// Method sudah dibatasi via WithValidMethods, jadi cukup return key.
		return m.key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("token tidak valid")
	}
	return claims, nil
}
