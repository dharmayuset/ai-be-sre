// Package ldap menyediakan klien LDAP untuk berinteraksi dengan FreeIPA.
//
// Ada 2 jenis koneksi:
//   - Service connection: bind pakai service account, untuk operasi admin
//     (search user, reset password user lain).
//   - User connection: bind pakai credential user untuk verifikasi password
//     atau ganti password sendiri.
//
// Setiap method mengembalikan error yang spesifik (lihat errors.go) supaya
// handler bisa men-translate jadi HTTP status code yang tepat.
package ldap

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strings"

	ldap "github.com/go-ldap/ldap/v3"

	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	"github.com/dharmayuset/ai-be-sre/backend/internal/models"
)

// Sentinel errors. Handler memakai errors.Is() untuk mengecek tipe.
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrPasswordPolicy     = errors.New("password tidak memenuhi password policy FreeIPA")
	ErrNoEmail            = errors.New("user tidak punya email terdaftar")
	ErrConnection         = errors.New("gagal connect ke LDAP server")
	ErrPermissionDenied   = errors.New("permission denied")
)

// Client adalah wrapper LDAP yang aman dipakai concurrent.
type Client struct {
	cfg *config.Config
}

// New membuat klien baru. Tidak membuka koneksi (lazy).
func New(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

// dial membuka koneksi baru ke LDAP server (1 koneksi per operasi).
// Pendekatan "connection per request" sengaja dipilih untuk simplicity
// & supaya error di satu koneksi tidak meracuni koneksi lain.
func (c *Client) dial() (*ldap.Conn, error) {
	url := c.cfg.LDAPServerURL
	var conn *ldap.Conn
	var err error

	if c.cfg.LDAPUseTLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: !c.cfg.LDAPVerifyCert, // #nosec G402 - controlled by config
			MinVersion:         tls.VersionTLS12,
		}
		conn, err = ldap.DialURL(url, ldap.DialWithTLSConfig(tlsCfg))
	} else {
		conn, err = ldap.DialURL(url)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnection, err)
	}
	return conn, nil
}

// dialAndBindService membuka koneksi & bind sebagai service account.
func (c *Client) dialAndBindService() (*ldap.Conn, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	if err := conn.Bind(c.cfg.LDAPBindDN, c.cfg.LDAPBindPassword); err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: bind service account gagal: %v", ErrConnection, err)
	}
	return conn, nil
}

// userDNFor membentuk DN user dari uid.
// Format FreeIPA standar: uid=<username>,<userBaseDN>.
func (c *Client) userDNFor(username string) string {
	return fmt.Sprintf("uid=%s,%s", ldap.EscapeDN(username), c.cfg.LDAPUserBaseDN)
}

// fetchUser mencari user berdasarkan uid pakai koneksi service.
// Return user lengkap dengan groups & isAdmin flag.
func (c *Client) fetchUser(conn *ldap.Conn, username string) (*models.User, error) {
	filter := fmt.Sprintf("(&(objectClass=person)(uid=%s))", ldap.EscapeFilter(username))
	req := ldap.NewSearchRequest(
		c.cfg.LDAPUserBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		1, 5, false,
		filter,
		[]string{"uid", "mail", "givenName", "sn", "cn", "displayName", "memberOf", "nsAccountLock"},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("ldap search failed: %w", err)
	}
	if len(res.Entries) == 0 {
		return nil, ErrUserNotFound
	}
	e := res.Entries[0]

	groups := extractGroupNames(e.GetAttributeValues("memberOf"))
	locked := strings.EqualFold(e.GetAttributeValue("nsAccountLock"), "true")

	u := &models.User{
		Username:    e.GetAttributeValue("uid"),
		DN:          e.DN,
		Email:       e.GetAttributeValue("mail"),
		DisplayName: firstNonEmpty(e.GetAttributeValue("displayName"), e.GetAttributeValue("cn")),
		FirstName:   e.GetAttributeValue("givenName"),
		LastName:    e.GetAttributeValue("sn"),
		Groups:      groups,
		Locked:      locked,
	}
	u.IsAdmin = containsCI(groups, c.cfg.LDAPAdminGroup)
	return u, nil
}

// Authenticate memverifikasi username + password user, return profile + role.
//
// Cara: bind sebagai user tersebut. Kalau berhasil = password benar.
// Lalu tutup koneksi user, dan ambil detail user pakai service account
// (supaya kita selalu dapat memberOf, dll).
func (c *Client) Authenticate(username, password string) (*models.User, error) {
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	// 1) Bind sebagai user untuk verifikasi password.
	userConn, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer userConn.Close()

	if err := userConn.Bind(c.userDNFor(username), password); err != nil {
		return nil, classifyBindError(err)
	}

	// 2) Ambil profil lengkap pakai service account.
	svcConn, err := c.dialAndBindService()
	if err != nil {
		return nil, err
	}
	defer svcConn.Close()

	user, err := c.fetchUser(svcConn, username)
	if err != nil {
		return nil, err
	}
	if user.Locked {
		return nil, ErrAccountLocked
	}
	return user, nil
}

// GetUser mengambil profil user. Dipakai admin untuk lihat detail user.
func (c *Client) GetUser(username string) (*models.User, error) {
	conn, err := c.dialAndBindService()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return c.fetchUser(conn, username)
}

// ListUsers mengembalikan daftar user (paged via filter sederhana).
// `query` opsional, mencari di uid/cn/mail (substring).
func (c *Client) ListUsers(query string, limit int) ([]*models.User, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	conn, err := c.dialAndBindService()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	filter := "(objectClass=person)"
	if q := strings.TrimSpace(query); q != "" {
		safe := ldap.EscapeFilter(q)
		filter = fmt.Sprintf(
			"(&(objectClass=person)(|(uid=*%s*)(cn=*%s*)(mail=*%s*)))",
			safe, safe, safe,
		)
	}

	req := ldap.NewSearchRequest(
		c.cfg.LDAPUserBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		limit, 10, false,
		filter,
		[]string{"uid", "mail", "givenName", "sn", "cn", "displayName", "memberOf", "nsAccountLock"},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("ldap search failed: %w", err)
	}

	out := make([]*models.User, 0, len(res.Entries))
	for _, e := range res.Entries {
		groups := extractGroupNames(e.GetAttributeValues("memberOf"))
		u := &models.User{
			Username:    e.GetAttributeValue("uid"),
			DN:          e.DN,
			Email:       e.GetAttributeValue("mail"),
			DisplayName: firstNonEmpty(e.GetAttributeValue("displayName"), e.GetAttributeValue("cn")),
			FirstName:   e.GetAttributeValue("givenName"),
			LastName:    e.GetAttributeValue("sn"),
			Groups:      groups,
			Locked:      strings.EqualFold(e.GetAttributeValue("nsAccountLock"), "true"),
			IsAdmin:     containsCI(groups, c.cfg.LDAPAdminGroup),
		}
		out = append(out, u)
	}
	return out, nil
}

// ChangePasswordSelf mengganti password user oleh user itu sendiri.
// Bind sebagai user, lalu kirim PasswordModifyRequest dengan oldPwd + newPwd.
// FreeIPA TIDAK akan menandai password expired untuk operasi ini.
func (c *Client) ChangePasswordSelf(username, oldPassword, newPassword string) error {
	if oldPassword == "" || newPassword == "" {
		return ErrInvalidCredentials
	}

	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	userDN := c.userDNFor(username)
	if err := conn.Bind(userDN, oldPassword); err != nil {
		return classifyBindError(err)
	}

	// Pakai Password Modify Extended Operation (RFC 3062)
	pmReq := ldap.NewPasswordModifyRequest(userDN, oldPassword, newPassword)
	if _, err := conn.PasswordModify(pmReq); err != nil {
		return classifyPasswordError(err)
	}
	return nil
}

// ResetPasswordAdmin mereset password user oleh service account.
//
// Karena reset dilakukan oleh akun lain, FreeIPA otomatis menandai
// password sebagai EXPIRED. User wajib ganti password saat login pertama.
// Inilah mekanisme "temporary password".
//
// Return: email user (untuk dipakai mengirim notifikasi).
func (c *Client) ResetPasswordAdmin(username, newPassword string) (string, error) {
	conn, err := c.dialAndBindService()
	if err != nil {
		return "", err
	}
	defer conn.Close()

	user, err := c.fetchUser(conn, username)
	if err != nil {
		return "", err
	}
	if user.Email == "" {
		return "", ErrNoEmail
	}

	// Pakai Password Modify Extended Operation tanpa old password.
	// Service account harus punya privilege "Modify Users password" / "User Administrators".
	pmReq := ldap.NewPasswordModifyRequest(user.DN, "", newPassword)
	if _, err := conn.PasswordModify(pmReq); err != nil {
		return "", classifyPasswordError(err)
	}
	return user.Email, nil
}

// SetUserLock mengunci/membuka akun user. Dipakai admin.
func (c *Client) SetUserLock(username string, lock bool) error {
	conn, err := c.dialAndBindService()
	if err != nil {
		return err
	}
	defer conn.Close()

	user, err := c.fetchUser(conn, username)
	if err != nil {
		return err
	}

	mod := ldap.NewModifyRequest(user.DN, nil)
	if lock {
		mod.Replace("nsAccountLock", []string{"TRUE"})
	} else {
		mod.Replace("nsAccountLock", []string{"FALSE"})
	}
	if err := conn.Modify(mod); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInsufficientAccessRights) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("modify failed: %w", err)
	}
	return nil
}

// DeleteUser menghapus user dari FreeIPA.
// PERHATIAN: operasi ini PERMANEN dan tidak bisa di-undo.
// Service account harus punya privilege "Remove Users".
func (c *Client) DeleteUser(username string) error {
	conn, err := c.dialAndBindService()
	if err != nil {
		return err
	}
	defer conn.Close()

	user, err := c.fetchUser(conn, username)
	if err != nil {
		return err
	}

	delReq := ldap.NewDelRequest(user.DN, nil)
	if err := conn.Del(delReq); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInsufficientAccessRights) {
			return ErrPermissionDenied
		}
		if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
			return ErrUserNotFound
		}
		return fmt.Errorf("delete user failed: %w", err)
	}
	return nil
}

// DeleteUsers menghapus banyak user sekaligus (batch).
// Return: map[username]error — nil berarti sukses, non-nil berarti gagal.
// Tetap lanjutkan walau ada yang gagal (partial success).
func (c *Client) DeleteUsers(usernames []string) map[string]error {
	results := make(map[string]error, len(usernames))
	for _, u := range usernames {
		results[u] = c.DeleteUser(u)
	}
	return results
}

// UserStats mengembalikan jumlah user aktif dan tidak aktif (locked).
func (c *Client) UserStats() (active, inactive, total int, err error) {
	conn, err := c.dialAndBindService()
	if err != nil {
		return 0, 0, 0, err
	}
	defer conn.Close()

	// Ambil semua user, hanya atribut nsAccountLock untuk efisiensi
	req := ldap.NewSearchRequest(
		c.cfg.LDAPUserBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		0, 30, false,
		"(objectClass=person)",
		[]string{"nsAccountLock"},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ldap search failed: %w", err)
	}

	total = len(res.Entries)
	for _, e := range res.Entries {
		if strings.EqualFold(e.GetAttributeValue("nsAccountLock"), "true") {
			inactive++
		} else {
			active++
		}
	}
	return active, inactive, total, nil
}

// ListUsersFiltered mengembalikan daftar user dengan filter status (active/inactive/all).
// `status`: "active", "inactive", atau "" (semua).
func (c *Client) ListUsersFiltered(query, status string, limit int) ([]*models.User, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	conn, err := c.dialAndBindService()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Base filter
	baseFilter := "(objectClass=person)"
	if q := strings.TrimSpace(query); q != "" {
		safe := ldap.EscapeFilter(q)
		baseFilter = fmt.Sprintf(
			"(&(objectClass=person)(|(uid=*%s*)(cn=*%s*)(mail=*%s*)))",
			safe, safe, safe,
		)
	}

	// Tambah filter status
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		// User yang TIDAK locked (nsAccountLock tidak exist atau bukan "true")
		baseFilter = fmt.Sprintf("(&%s(!(nsAccountLock=TRUE)))", baseFilter)
	case "inactive", "locked":
		// User yang locked
		baseFilter = fmt.Sprintf("(&%s(nsAccountLock=TRUE))", baseFilter)
	}

	req := ldap.NewSearchRequest(
		c.cfg.LDAPUserBaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		limit, 30, false,
		baseFilter,
		[]string{"uid", "mail", "givenName", "sn", "cn", "displayName", "memberOf", "nsAccountLock"},
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return nil, fmt.Errorf("ldap search failed: %w", err)
	}

	out := make([]*models.User, 0, len(res.Entries))
	for _, e := range res.Entries {
		groups := extractGroupNames(e.GetAttributeValues("memberOf"))
		u := &models.User{
			Username:    e.GetAttributeValue("uid"),
			DN:          e.DN,
			Email:       e.GetAttributeValue("mail"),
			DisplayName: firstNonEmpty(e.GetAttributeValue("displayName"), e.GetAttributeValue("cn")),
			FirstName:   e.GetAttributeValue("givenName"),
			LastName:    e.GetAttributeValue("sn"),
			Groups:      groups,
			Locked:      strings.EqualFold(e.GetAttributeValue("nsAccountLock"), "true"),
			IsAdmin:     containsCI(groups, c.cfg.LDAPAdminGroup),
		}
		out = append(out, u)
	}
	return out, nil
}

// ---------- error classification ----------

// classifyBindError memetakan error bind ke sentinel error yang lebih bermakna.
// FreeIPA mengembalikan beberapa code yang bisa kita translate.
func classifyBindError(err error) error {
	if err == nil {
		return nil
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
		return ErrInvalidCredentials
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultUnwillingToPerform) {
		// FreeIPA pakai 53 untuk akun locked / disabled
		return ErrAccountLocked
	}
	return fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
}

func classifyPasswordError(err error) error {
	if err == nil {
		return nil
	}
	// Constraint violation = password tidak lulus policy
	if ldap.IsErrorWithCode(err, ldap.LDAPResultConstraintViolation) {
		return ErrPasswordPolicy
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultInsufficientAccessRights) {
		return ErrPermissionDenied
	}
	if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
		return ErrInvalidCredentials
	}
	return fmt.Errorf("password modify failed: %w", err)
}

// ---------- helpers ----------

// extractGroupNames memetakan slice DN grup -> slice nama grup (cn).
// Misal "cn=admins,cn=groups,cn=accounts,dc=example,dc=com" -> "admins".
func extractGroupNames(memberOf []string) []string {
	out := make([]string, 0, len(memberOf))
	for _, dn := range memberOf {
		parsed, err := ldap.ParseDN(dn)
		if err != nil || len(parsed.RDNs) == 0 {
			continue
		}
		// RDN pertama biasanya cn=<group>
		for _, av := range parsed.RDNs[0].Attributes {
			if strings.EqualFold(av.Type, "cn") {
				out = append(out, av.Value)
				break
			}
		}
	}
	return out
}

func containsCI(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
