package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/config"
	"github.com/firblab-blog/jobhunt-os/internal/server"
	"github.com/firblab-blog/jobhunt-os/internal/store/sqlite"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		slog.Error("invalid runtime config", "error", err)
		os.Exit(1)
	}

	db, dbPath, err := sqlite.Open(context.Background(), cfg.DataDir)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	if err := sqlite.Migrate(context.Background(), db); err != nil {
		slog.Error("migrate database", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: server.NewWithOptions(sqlite.NewStore(db), server.Options{
			DataDir:       cfg.DataDir,
			SecureCookies: cfg.SecureCookies,
			Auth: server.AuthOptions{
				Username:     cfg.AuthUsername,
				PasswordHash: cfg.AuthPasswordHash,
			},
		}),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	slog.Info("starting jobhunt-os", "addr", cfg.Addr, "allow_network", cfg.AllowNetwork, "db_path", dbPath)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	slog.Info("shutting down jobhunt-os")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
