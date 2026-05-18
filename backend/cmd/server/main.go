// Command server adalah entry point HTTP server.
//
// Flow:
//  1. Load config (.env / env vars). Fail-fast kalau invalid.
//  2. Wire dependencies: logger, audit DB, LDAP client, mailer, JWT manager.
//  3. Build router dengan middleware & handlers.
//  4. Start HTTP server dengan timeouts yang aman.
//  5. Graceful shutdown saat SIGTERM/SIGINT.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/dharmayuset/ai-be-sre/backend/internal/audit"
	"github.com/dharmayuset/ai-be-sre/backend/internal/auth"
	"github.com/dharmayuset/ai-be-sre/backend/internal/config"
	"github.com/dharmayuset/ai-be-sre/backend/internal/email"
	"github.com/dharmayuset/ai-be-sre/backend/internal/handlers"
	ldapsvc "github.com/dharmayuset/ai-be-sre/backend/internal/ldap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := buildLogger(cfg)
	logger.Info("starting",
		slog.String("app", cfg.AppName),
		slog.String("env", cfg.AppEnv),
	)

	// Audit log DB
	auditDB, err := audit.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("init audit db: %w", err)
	}
	defer auditDB.Close()

	// Wire services
	ldapClient := ldapsvc.New(cfg)
	mailer := email.New(cfg)
	jwtMgr := auth.NewManager(cfg)

	// Wire handlers
	authH := handlers.NewAuthHandler(cfg, jwtMgr, ldapClient, auditDB, logger)
	pwdH := handlers.NewPasswordHandler(cfg, ldapClient, mailer, auditDB, logger)
	adminH := handlers.NewAdminHandler(cfg, ldapClient, mailer, auditDB, logger)

	router := buildRouter(cfg, logger, authH, pwdH, adminH, jwtMgr)

	addr := net.JoinHostPort(cfg.AppHost, strconv.Itoa(cfg.AppPort))
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Run server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("listening", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-sigCh:
		logger.Info("shutdown signal received", slog.String("sig", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	logger.Info("bye")
	return nil
}

// buildLogger returns a slog logger with format depending on env.
// Production: JSON (machine-readable). Development: text (human-readable).
func buildLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	opts := &slog.HandlerOptions{Level: level}
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
