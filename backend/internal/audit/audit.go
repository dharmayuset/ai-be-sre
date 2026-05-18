// Package audit menyediakan persistent audit log untuk semua aktivitas
// sensitif (login, change password, reset password, lock/unlock).
//
// Storage: SQLite (file-based, no server needed). Cocok untuk skala kecil-sedang.
// Untuk skala besar, ganti dengan Postgres / kirim ke central log system.
package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Action adalah jenis event yang di-audit. Pakai konstanta supaya
// konsisten antar handler.
type Action string

const (
	ActionLogin          Action = "login"
	ActionLoginFailed    Action = "login_failed"
	ActionLogout         Action = "logout"
	ActionChangePassword Action = "change_password"
	ActionResetPassword  Action = "reset_password"
	ActionAdminReset     Action = "admin_reset_password"
	ActionLockUser       Action = "admin_lock_user"
	ActionUnlockUser     Action = "admin_unlock_user"
)

// Status apakah aksi berhasil atau gagal.
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
)

// Entry adalah record audit yang di-store dan di-return ke admin.
type Entry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`  // username yang melakukan aksi (kalau anonymous: "-")
	Target    string    `json:"target"` // username target (kalau aksi terhadap user lain)
	Action    Action    `json:"action"`
	Status    Status    `json:"status"`
	IPAddress string    `json:"ipAddress"`
	UserAgent string    `json:"userAgent"`
	Message   string    `json:"message,omitempty"` // detail tambahan / error message
}

// Logger interface — handler depend on this, bukan struct konkret.
type Logger interface {
	Log(ctx context.Context, e Entry) error
	List(ctx context.Context, filter ListFilter) ([]Entry, int, error)
	Stats(ctx context.Context, since time.Time) (Stats, error)
	Close() error
}

// ListFilter parameter untuk query audit log.
type ListFilter struct {
	Actor  string
	Target string
	Action string
	Status string
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
}

// Stats agregat untuk dashboard admin.
type Stats struct {
	TotalEvents    int64            `json:"totalEvents"`
	SuccessCount   int64            `json:"successCount"`
	FailureCount   int64            `json:"failureCount"`
	ByAction       map[string]int64 `json:"byAction"`
	RecentFailures int64            `json:"recentFailures"` // gagal dalam 24h terakhir
	ResetCount     int64            `json:"resetCount"`     // reset password 24h terakhir
}

// Service implementasi Logger pakai SQLite.
type Service struct {
	db *sql.DB
}

// New membuka (atau membuat) database SQLite dan menjalankan migrasi.
func New(dbPath string) (*Service, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	// _journal_mode=WAL = better concurrent reads
	// _foreign_keys=on  = enforce relational constraints
	// _busy_timeout=5000 = wait kalau ada lock (5s)
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000&_synchronous=NORMAL", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite + WAL biasanya cukup dengan max 1 writer; reads ok concurrent.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Service{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Service) migrate() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS audit_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          DATETIME NOT NULL,
    actor       TEXT NOT NULL DEFAULT '-',
    target      TEXT NOT NULL DEFAULT '-',
    action      TEXT NOT NULL,
    status      TEXT NOT NULL,
    ip_address  TEXT NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    message     TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_audit_ts     ON audit_logs(ts DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor  ON audit_logs(actor);
CREATE INDEX IF NOT EXISTS idx_audit_target ON audit_logs(target);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
`
	_, err := s.db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("migrate audit_logs: %w", err)
	}
	return nil
}

// Close menutup koneksi DB.
func (s *Service) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Log menulis 1 entry audit.
func (s *Service) Log(ctx context.Context, e Entry) error {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.Actor == "" {
		e.Actor = "-"
	}
	if e.Target == "" {
		e.Target = "-"
	}

	const q = `INSERT INTO audit_logs
		(ts, actor, target, action, status, ip_address, user_agent, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q,
		e.Timestamp.UTC(), e.Actor, e.Target, string(e.Action),
		string(e.Status), e.IPAddress, e.UserAgent, e.Message,
	)
	if err != nil {
		return fmt.Errorf("audit insert: %w", err)
	}
	return nil
}

// List mengembalikan audit log dengan filter & pagination.
// Return: (entries, totalCount, error). totalCount untuk pagination UI.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Entry, int, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 50
	}

	// Build WHERE dynamically supaya query optimal.
	where := "WHERE 1=1"
	args := []any{}

	if f.Actor != "" {
		where += " AND actor = ?"
		args = append(args, f.Actor)
	}
	if f.Target != "" {
		where += " AND target = ?"
		args = append(args, f.Target)
	}
	if f.Action != "" {
		where += " AND action = ?"
		args = append(args, f.Action)
	}
	if f.Status != "" {
		where += " AND status = ?"
		args = append(args, f.Status)
	}
	if !f.From.IsZero() {
		where += " AND ts >= ?"
		args = append(args, f.From.UTC())
	}
	if !f.To.IsZero() {
		where += " AND ts <= ?"
		args = append(args, f.To.UTC())
	}

	// Total count
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_logs "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit count: %w", err)
	}

	// Page query
	q := "SELECT id, ts, actor, target, action, status, ip_address, user_agent, message FROM audit_logs " +
		where + " ORDER BY ts DESC LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit list: %w", err)
	}
	defer rows.Close()

	out := make([]Entry, 0, f.Limit)
	for rows.Next() {
		var e Entry
		var action, status string
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Actor, &e.Target,
			&action, &status, &e.IPAddress, &e.UserAgent, &e.Message); err != nil {
			return nil, 0, fmt.Errorf("audit scan: %w", err)
		}
		e.Action = Action(action)
		e.Status = Status(status)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Stats menghitung agregat untuk dashboard admin.
func (s *Service) Stats(ctx context.Context, since time.Time) (Stats, error) {
	stats := Stats{ByAction: make(map[string]int64)}

	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failure' THEN 1 ELSE 0 END), 0)
		FROM audit_logs
	`)
	if err := row.Scan(&stats.TotalEvents, &stats.SuccessCount, &stats.FailureCount); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return stats, fmt.Errorf("stats overall: %w", err)
		}
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT action, COUNT(*) FROM audit_logs GROUP BY action
	`)
	if err != nil {
		return stats, fmt.Errorf("stats by action: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a string
		var c int64
		if err := rows.Scan(&a, &c); err != nil {
			return stats, err
		}
		stats.ByAction[a] = c
	}

	// Counter 24h terakhir
	since24h := time.Now().Add(-24 * time.Hour)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE status='failure' AND ts >= ?`, since24h,
	).Scan(&stats.RecentFailures); err != nil {
		return stats, fmt.Errorf("stats recent failures: %w", err)
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_logs WHERE action IN (?, ?) AND ts >= ?`,
		string(ActionResetPassword), string(ActionAdminReset), since24h,
	).Scan(&stats.ResetCount); err != nil {
		return stats, fmt.Errorf("stats reset count: %w", err)
	}

	_ = since // reserved for future per-period stats
	return stats, nil
}
