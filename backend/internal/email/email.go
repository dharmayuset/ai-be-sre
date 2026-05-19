// Package email mengirim email via SMTP relay.
//
// Mendukung 3 mode koneksi:
//   - Plain (port 25): biasanya untuk relay internal yang trust by IP.
//   - STARTTLS (port 587): koneksi mulai plain lalu upgrade ke TLS.
//   - Implicit TLS / SMTPS (port 465): TLS langsung dari awal.
//
// Pilihan mode dikontrol via SMTP_USE_STARTTLS / SMTP_USE_TLS di config.
//
// Untuk SMTP relay tanpa auth (skenario umum di internal network),
// kosongkan SMTP_USERNAME dan SMTP_PASSWORD; client akan skip AUTH.
package email

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
)

// Sentinel errors.
var (
	ErrInvalidRecipient = errors.New("invalid recipient email")
	ErrSendFailed       = errors.New("failed to send email")
)

// Sender adalah interface yang dipakai handler. Memudahkan testing
// (bisa di-stub) dan dependency injection.
type Sender interface {
	SendTemporaryPassword(to, username, tempPassword string) error
	SendPasswordChanged(to, username string) error
}

// Service adalah implementasi default Sender pakai SMTP.
type Service struct {
	cfg *config.Config
}

// New membuat email service. Tidak membuka koneksi (lazy per-send).
func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// SendTemporaryPassword mengirim email berisi temporary password.
func (s *Service) SendTemporaryPassword(to, username, tempPassword string) error {
	subject := "Reset Password - Temporary Password Anda"

	data := map[string]any{
		"Username":     username,
		"TempPassword": tempPassword,
		"FromName":     s.cfg.SMTPFromName,
		"Year":         time.Now().Year(),
	}
	textBody, err := renderText(tmplTempPasswordText, data)
	if err != nil {
		return err
	}
	htmlBody, err := renderHTML(tmplTempPasswordHTML, data)
	if err != nil {
		return err
	}
	return s.send(to, subject, textBody, htmlBody)
}

// SendPasswordChanged mengirim notifikasi bahwa password baru saja diganti.
// Berfungsi sebagai security alert untuk user.
func (s *Service) SendPasswordChanged(to, username string) error {
	subject := "Password Anda Baru Saja Diubah"
	data := map[string]any{
		"Username": username,
		"FromName": s.cfg.SMTPFromName,
		"Time":     time.Now().Format("2 Jan 2006, 15:04 MST"),
		"Year":     time.Now().Year(),
	}
	textBody, err := renderText(tmplPasswordChangedText, data)
	if err != nil {
		return err
	}
	htmlBody, err := renderHTML(tmplPasswordChangedHTML, data)
	if err != nil {
		return err
	}
	return s.send(to, subject, textBody, htmlBody)
}

// ---------- core send ----------

// send melakukan koneksi SMTP relay & mengirim message multipart.
func (s *Service) send(to, subject, textBody, htmlBody string) error {
	addr, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRecipient, err)
	}
	from := mail.Address{Name: s.cfg.SMTPFromName, Address: s.cfg.SMTPFromEmail}

	msg, err := buildMultipartMessage(from, *addr, subject, textBody, htmlBody)
	if err != nil {
		return fmt.Errorf("build message: %w", err)
	}

	host := s.cfg.SMTPHost
	port := s.cfg.SMTPPort
	serverAddr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	dialer := &net.Dialer{Timeout: s.cfg.SMTPTimeout}

	var (
		conn    net.Conn
		client  *smtp.Client
		dialErr error
	)

	tlsCfg := &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: s.cfg.SMTPSkipVerify, // #nosec G402 - controlled by config (dev only)
	}

	switch {
	case s.cfg.SMTPUseTLS:
		// Implicit TLS (SMTPS, port 465 biasanya)
		conn, dialErr = tls.DialWithDialer(dialer, "tcp", serverAddr, tlsCfg)
	default:
		// Plain TCP (akan di-upgrade kalau STARTTLS aktif)
		conn, dialErr = dialer.Dial("tcp", serverAddr)
	}
	if dialErr != nil {
		return fmt.Errorf("%w: dial: %v", ErrSendFailed, dialErr)
	}
	// Pastikan connection di-close.
	defer conn.Close()

	client, err = smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("%w: smtp client: %v", ErrSendFailed, err)
	}
	defer client.Close()

	// Set deadline supaya hang tidak block forever.
	_ = conn.SetDeadline(time.Now().Add(s.cfg.SMTPTimeout))

	if err := client.Hello(safeHostname()); err != nil {
		return fmt.Errorf("%w: EHLO: %v", ErrSendFailed, err)
	}

	// STARTTLS upgrade (kalau dipilih dan belum dalam TLS)
	if !s.cfg.SMTPUseTLS && s.cfg.SMTPUseSTARTTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("%w: starttls: %v", ErrSendFailed, err)
			}
		} else {
			// Server tidak announce STARTTLS — di production sebaiknya fail.
			// Di dev/internal relay, kita biarkan jalan plaintext.
		}
	}

	// Auth (opsional) — banyak SMTP relay internal trust by IP, no auth.
	if s.cfg.SMTPUsername != "" && s.cfg.SMTPPassword != "" {
		if ok, _ := client.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("%w: auth: %v", ErrSendFailed, err)
			}
		}
	}

	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("%w: MAIL FROM: %v", ErrSendFailed, err)
	}
	if err := client.Rcpt(addr.Address); err != nil {
		return fmt.Errorf("%w: RCPT TO: %v", ErrSendFailed, err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("%w: DATA: %v", ErrSendFailed, err)
	}
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return fmt.Errorf("%w: write body: %v", ErrSendFailed, err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("%w: close body: %v", ErrSendFailed, err)
	}

	if err := client.Quit(); err != nil {
		// QUIT error tidak fatal (server kadang tutup duluan).
		_ = err
	}
	return nil
}

// ---------- message builder ----------

// buildMultipartMessage menyusun email RFC 2045/2046 dengan
// alternative text + html body.
func buildMultipartMessage(from, to mail.Address, subject, text, html string) ([]byte, error) {
	var buf bytes.Buffer
	boundary := fmt.Sprintf("=_boundary_%d", time.Now().UnixNano())

	// Headers
	buf.WriteString("From: " + from.String() + "\r\n")
	buf.WriteString("To: " + to.String() + "\r\n")
	buf.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	buf.WriteString("\r\n")

	// Plain text part
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	buf.WriteString(text)
	buf.WriteString("\r\n")

	// HTML part
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	buf.WriteString(html)
	buf.WriteString("\r\n")

	// End boundary
	buf.WriteString("--" + boundary + "--\r\n")
	return buf.Bytes(), nil
}

func renderText(tmpl string, data any) (string, error) {
	t, err := template.New("text").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderHTML(tmpl string, data any) (string, error) {
	// html/template otomatis escape data (cegah injection di body email)
	t, err := template.New("html").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func safeHostname() string {
	if h, err := net.LookupCNAME("localhost"); err == nil && h != "" {
		return strings.TrimSuffix(h, ".")
	}
	return "localhost"
}
