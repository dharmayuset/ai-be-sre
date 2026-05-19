// Package pritunl menyediakan klien untuk berkomunikasi dengan Pritunl VPN API.
//
// Pritunl API menggunakan autentikasi HMAC:
//   - Header Auth-Token = API token
//   - Header Auth-Timestamp = unix timestamp (string)
//   - Header Auth-Nonce = random UUID
//   - Header Auth-Signature = HMAC-SHA256(auth_token&timestamp&nonce&method&path, secret)
//
// Endpoint yang kita pakai:
//   - GET /user/{org_id}  → list users (cari berdasarkan email)
//   - GET /key/{org_id}/{user_id}.tar → download VPN profile (tar archive)
//
// Referensi: https://docs.pritunl.com/docs/api
package pritunl

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
)

// Sentinel errors.
var (
	ErrUserNotFound    = errors.New("user tidak ditemukan di Pritunl")
	ErrNoOrganization  = errors.New("tidak ada organization yang dikonfigurasi")
	ErrConnection      = errors.New("gagal connect ke Pritunl server")
	ErrProfileDownload = errors.New("gagal download VPN profile")
)

// PritunlUser adalah representasi user dari Pritunl API response.
type PritunlUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	OrgID    string `json:"organization"`
	OrgName  string `json:"organization_name"`
	Type     string `json:"type"`
	Disabled bool   `json:"disabled"`
}

// Client berkomunikasi dengan Pritunl API.
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

// New membuat Pritunl API client.
func New(cfg *config.Config) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.PritunlSkipTLSVerify, // #nosec G402 - controlled by config
			MinVersion:         tls.VersionTLS12,
		},
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
	}
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// FindUserByEmail mencari user di semua organization berdasarkan email.
// Return: PritunlUser pertama yang cocok.
func (c *Client) FindUserByEmail(email string) (*PritunlUser, error) {
	if email == "" {
		return nil, ErrUserNotFound
	}

	orgIDs := c.cfg.PritunlOrgIDs
	if len(orgIDs) == 0 {
		return nil, ErrNoOrganization
	}

	email = strings.ToLower(strings.TrimSpace(email))

	// Cari di setiap organization
	for _, orgID := range orgIDs {
		users, err := c.listUsersInOrg(orgID)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			if strings.EqualFold(u.Email, email) && !u.Disabled {
				u.OrgID = orgID
				return &u, nil
			}
		}
	}
	return nil, ErrUserNotFound
}

// DownloadProfile mengunduh VPN profile (.tar) untuk user tertentu.
// Return: raw bytes dari file tar.
func (c *Client) DownloadProfile(orgID, userID string) ([]byte, string, error) {
	path := fmt.Sprintf("/key/%s/%s.tar", orgID, userID)

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrConnection, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: status %d", ErrProfileDownload, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // max 10MB
	if err != nil {
		return nil, "", fmt.Errorf("%w: read body: %v", ErrProfileDownload, err)
	}

	// Filename dari Content-Disposition atau fallback
	filename := "vpn-profile.tar"
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if i := strings.Index(cd, "filename="); i >= 0 {
			name := strings.Trim(cd[i+9:], "\" ")
			if name != "" {
				filename = name
			}
		}
	}

	return data, filename, nil
}

// listUsersInOrg mengambil daftar user di organization tertentu.
func (c *Client) listUsersInOrg(orgID string) ([]PritunlUser, error) {
	path := fmt.Sprintf("/user/%s", orgID)

	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnection, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: list users status %d", ErrConnection, resp.StatusCode)
	}

	var users []PritunlUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decode users response: %w", err)
	}
	return users, nil
}

// doRequest membuat HTTP request dengan autentikasi HMAC Pritunl.
func (c *Client) doRequest(method, path string, body io.Reader) (*http.Response, error) {
	url := strings.TrimRight(c.cfg.PritunlBaseURL, "/") + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	// Auth headers
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := uuid.NewString()

	// Signature: HMAC-SHA256 dari "token&timestamp&nonce&METHOD&path"
	authString := strings.Join([]string{
		c.cfg.PritunlAPIToken,
		timestamp,
		nonce,
		strings.ToUpper(method),
		path,
	}, "&")

	mac := hmac.New(sha256.New, []byte(c.cfg.PritunlAPISecret))
	mac.Write([]byte(authString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("Auth-Token", c.cfg.PritunlAPIToken)
	req.Header.Set("Auth-Timestamp", timestamp)
	req.Header.Set("Auth-Nonce", nonce)
	req.Header.Set("Auth-Signature", signature)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}
