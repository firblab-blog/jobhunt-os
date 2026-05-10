package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/firblab-blog/jobhunt-os/internal/config"
)

func TestLogStartupSecurityWarningsLogsInsecureNoAuthEscapeHatch(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)

	logStartupSecurityWarnings(config.Config{
		Addr:                "0.0.0.0:8080",
		AuthMode:            config.AuthModeDisabled,
		AllowInsecureNoAuth: true,
	})

	got := logs.String()
	for _, want := range []string{
		"INSECURE no-auth mode allowed by escape hatch",
		"addr=0.0.0.0:8080",
		"auth_mode=disabled",
		"escape_hatch=" + config.EnvAllowInsecureNoAuth,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("logs do not contain %q: %s", want, got)
		}
	}
}

func TestLogStartupSecurityWarningsQuietForNormalModes(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(previous)

	logStartupSecurityWarnings(config.Config{
		Addr:     "127.0.0.1:8080",
		AuthMode: config.AuthModeDisabled,
	})
	logStartupSecurityWarnings(config.Config{
		Addr:                "0.0.0.0:8080",
		AuthMode:            config.AuthModeLogin,
		AllowInsecureNoAuth: true,
	})

	if got := logs.String(); got != "" {
		t.Fatalf("logs = %q, want empty", got)
	}
}
