// Package config memuat konfigurasi aplikasi dari environment variables.
//
// Semua setting di-load saat startup. Jika ada yang invalid, aplikasi
// fail-fast (panic) supaya kita tidak deploy dengan config yang salah.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config adalah seluruh setting aplikasi yang sudah di-parse & divalidasi.
type Config struct {
	// Server
	AppName            string
	AppEnv             string // development | production
	AppHost            string
	AppPort            int
	CORSAllowedOrigins []string

	// JWT
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	JWTIssuer     string

	// LDAP
	LDAPServerURL    string
	LDAPUserBaseDN   string
	LDAPGroupBaseDN  string
	LDAPBindDN       string
	LDAPBindPassword string
	LDAPAdminGroup   string
	LDAPUseTLS       bool
	LDAPVerifyCert   bool

	// SMTP
	SMTPHost        string
	SMTPPort        int
	SMTPUsername    string
	SMTPPassword    string
	SMTPUseSTARTTLS bool
	SMTPUseTLS      bool
	SMTPSkipVerify  bool
	SMTPFromEmail   string
	SMTPFromName    string
	SMTPTimeout     time.Duration

	// DB
	DBPath string

	// Pritunl VPN
	PritunlBaseURL       string
	PritunlAPIToken      string
	PritunlAPISecret     string
	PritunlOrgIDs        []string // comma-separated organization IDs
	PritunlSkipTLSVerify bool     // true = skip TLS verify (self-signed cert)

	// Security
	TempPasswordLength int
	RateLimitResetPM   int
	RateLimitAPIPM     int
	RateLimitLoginPM   int
	RateLimitVPNPM     int
}

// Load membaca .env (jika ada) lalu environment variables.
// Return error kalau ada konfigurasi wajib yang tidak terpenuhi.
func Load() (*Config, error) {
	// .env opsional: production biasanya inject env via systemd / k8s secrets.
	_ = godotenv.Load()

	cfg := &Config{
		AppName:            getEnv("APP_NAME", "FreeIPA Self-Service Portal"),
		AppEnv:             getEnv("APP_ENV", "development"),
		AppHost:            getEnv("APP_HOST", "0.0.0.0"),
		AppPort:            getEnvInt("APP_PORT", 8080),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),

		JWTSecret:     getEnv("JWT_SECRET", ""),
		JWTAccessTTL:  time.Duration(getEnvInt("JWT_ACCESS_TTL_MINUTES", 15)) * time.Minute,
		JWTRefreshTTL: time.Duration(getEnvInt("JWT_REFRESH_TTL_HOURS", 8)) * time.Hour,
		JWTIssuer:     getEnv("JWT_ISSUER", "ai-be-sre"),

		LDAPServerURL:    getEnv("LDAP_SERVER_URL", ""),
		LDAPUserBaseDN:   getEnv("LDAP_USER_BASE_DN", ""),
		LDAPGroupBaseDN:  getEnv("LDAP_GROUP_BASE_DN", ""),
		LDAPBindDN:       getEnv("LDAP_BIND_DN", ""),
		LDAPBindPassword: getEnv("LDAP_BIND_PASSWORD", ""),
		LDAPAdminGroup:   getEnv("LDAP_ADMIN_GROUP", "admins"),
		LDAPUseTLS:       getEnvBool("LDAP_USE_TLS", true),
		LDAPVerifyCert:   getEnvBool("LDAP_VERIFY_CERT", true),

		SMTPHost:        getEnv("SMTP_HOST", ""),
		SMTPPort:        getEnvInt("SMTP_PORT", 25),
		SMTPUsername:    getEnv("SMTP_USERNAME", ""),
		SMTPPassword:    getEnv("SMTP_PASSWORD", ""),
		SMTPUseSTARTTLS: getEnvBool("SMTP_USE_STARTTLS", true),
		SMTPUseTLS:      getEnvBool("SMTP_USE_TLS", false),
		SMTPSkipVerify:  getEnvBool("SMTP_SKIP_VERIFY", false),
		SMTPFromEmail:   getEnv("SMTP_FROM_EMAIL", ""),
		SMTPFromName:    getEnv("SMTP_FROM_NAME", "IT Support"),
		SMTPTimeout:     time.Duration(getEnvInt("SMTP_TIMEOUT_SECONDS", 15)) * time.Second,

		DBPath: getEnv("DB_PATH", "./data/audit.db"),

		PritunlBaseURL:       getEnv("PRITUNL_BASE_URL", ""),
		PritunlAPIToken:      getEnv("PRITUNL_API_TOKEN", ""),
		PritunlAPISecret:     getEnv("PRITUNL_API_SECRET", ""),
		PritunlOrgIDs:        splitCSV(getEnv("PRITUNL_ORG_IDS", "")),
		PritunlSkipTLSVerify: getEnvBool("PRITUNL_SKIP_TLS_VERIFY", false),

		TempPasswordLength: getEnvInt("TEMP_PASSWORD_LENGTH", 16),
		RateLimitResetPM:   getEnvInt("RATE_LIMIT_RESET_PER_MIN", 5),
		RateLimitAPIPM:     getEnvInt("RATE_LIMIT_API_PER_MIN", 60),
		RateLimitLoginPM:   getEnvInt("RATE_LIMIT_LOGIN_PER_MIN", 10),
		RateLimitVPNPM:     getEnvInt("RATE_LIMIT_VPN_PER_MIN", 3),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// IsProduction true jika APP_ENV=production.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

func (c *Config) validate() error {
	var errs []string

	must := func(name, val string) {
		if strings.TrimSpace(val) == "" {
			errs = append(errs, fmt.Sprintf("%s wajib di-set", name))
		}
	}

	// Production wajib pakai secret yang kuat
	if c.IsProduction() {
		if len(c.JWTSecret) < 32 {
			errs = append(errs, "JWT_SECRET minimal 32 karakter di production")
		}
		if c.JWTSecret == "change-me-please-use-a-very-long-random-secret-min-64-chars" {
			errs = append(errs, "JWT_SECRET masih default, harus diganti di production")
		}
	} else if c.JWTSecret == "" {
		errs = append(errs, "JWT_SECRET wajib di-set")
	}

	must("LDAP_SERVER_URL", c.LDAPServerURL)
	must("LDAP_USER_BASE_DN", c.LDAPUserBaseDN)
	must("LDAP_GROUP_BASE_DN", c.LDAPGroupBaseDN)
	must("LDAP_BIND_DN", c.LDAPBindDN)
	must("LDAP_BIND_PASSWORD", c.LDAPBindPassword)
	must("LDAP_ADMIN_GROUP", c.LDAPAdminGroup)

	must("SMTP_HOST", c.SMTPHost)
	must("SMTP_FROM_EMAIL", c.SMTPFromEmail)

	if c.TempPasswordLength < 12 {
		errs = append(errs, "TEMP_PASSWORD_LENGTH minimal 12 untuk security")
	}

	if c.SMTPUseTLS && c.SMTPUseSTARTTLS {
		errs = append(errs, "SMTP_USE_TLS dan SMTP_USE_STARTTLS tidak boleh keduanya true")
	}

	if len(errs) > 0 {
		return errors.New("config invalid:\n  - " + strings.Join(errs, "\n  - "))
	}
	return nil
}

// ---------- helpers ----------

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		switch strings.ToLower(v) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return def
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
