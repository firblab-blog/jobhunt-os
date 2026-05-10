package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/auth"
)

func TestLoadDefaultsToLoopback(t *testing.T) {
	t.Parallel()

	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Addr != DefaultAddr {
		t.Fatalf("Addr = %q, want %q", cfg.Addr, DefaultAddr)
	}
	if cfg.AllowNetwork {
		t.Fatalf("AllowNetwork = true, want false")
	}
	if cfg.DataDir == "" {
		t.Fatalf("DataDir is empty")
	}
	if cfg.AuthUsername != "" || cfg.AuthPasswordHash != "" {
		t.Fatalf("auth config = %q/%q, want empty", cfg.AuthUsername, cfg.AuthPasswordHash)
	}
	if cfg.AuthMode != AuthModeDisabled {
		t.Fatalf("AuthMode = %q, want %q", cfg.AuthMode, AuthModeDisabled)
	}
	if cfg.AllowInsecureNoAuth {
		t.Fatalf("AllowInsecureNoAuth = true, want false")
	}
	if cfg.SessionIdleTimeout != 12*time.Hour {
		t.Fatalf("SessionIdleTimeout = %s, want 12h", cfg.SessionIdleTimeout)
	}
	if cfg.SessionAbsoluteTimeout != 30*24*time.Hour {
		t.Fatalf("SessionAbsoluteTimeout = %s, want 30d", cfg.SessionAbsoluteTimeout)
	}
	if cfg.AuthTrustProxyHeaders {
		t.Fatalf("AuthTrustProxyHeaders = true, want false")
	}
	if cfg.SecureCookies {
		t.Fatalf("SecureCookies = true, want false")
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Fatalf("ReadTimeout = %s, want 5s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %s, want 10s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 2*time.Minute {
		t.Fatalf("IdleTimeout = %s, want 2m", cfg.IdleTimeout)
	}
}

func TestLoadParsesEnv(t *testing.T) {
	t.Parallel()

	cfg, err := Load(env(map[string]string{
		EnvAddr:         "localhost:9090",
		EnvAllowNetwork: "true",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Addr != "localhost:9090" {
		t.Fatalf("Addr = %q, want localhost:9090", cfg.Addr)
	}
	if !cfg.AllowNetwork {
		t.Fatalf("AllowNetwork = false, want true")
	}
}

func TestLoadParsesSecureCookiesEnv(t *testing.T) {
	t.Parallel()

	cfg, err := Load(env(map[string]string{
		EnvSecureCookies: "true",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.SecureCookies {
		t.Fatalf("SecureCookies = false, want true")
	}
}

func TestLoadRejectsInvalidSecureCookies(t *testing.T) {
	t.Parallel()

	if _, err := Load(env(map[string]string{EnvSecureCookies: "sometimes"})); err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
}

func TestLoadParsesAuthEnv(t *testing.T) {
	t.Parallel()

	passwordHash := testPasswordHash(t)
	cfg, err := Load(env(map[string]string{
		EnvAuthUsername:     " avery ",
		EnvAuthPasswordHash: " " + passwordHash + " ",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AuthUsername != "avery" {
		t.Fatalf("AuthUsername = %q, want avery", cfg.AuthUsername)
	}
	if cfg.AuthMode != AuthModeBasic {
		t.Fatalf("AuthMode = %q, want %q", cfg.AuthMode, AuthModeBasic)
	}
	if cfg.AuthPasswordHash != passwordHash {
		t.Fatalf("AuthPasswordHash = %q, want configured hash", cfg.AuthPasswordHash)
	}
}

func TestLoadParsesAuthPasswordFile(t *testing.T) {
	t.Parallel()

	password := "correct horse battery staple"
	passwordFile := filepath.Join(t.TempDir(), "admin-password")
	if err := os.WriteFile(passwordFile, []byte(password+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(env(map[string]string{
		EnvAuthMode:         AuthModeLogin,
		EnvAuthUsername:     "avery",
		EnvAuthPasswordFile: passwordFile,
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AuthUsername != "avery" {
		t.Fatalf("AuthUsername = %q, want avery", cfg.AuthUsername)
	}
	if cfg.AuthMode != AuthModeLogin {
		t.Fatalf("AuthMode = %q, want %q", cfg.AuthMode, AuthModeLogin)
	}
	passwordHash, err := auth.ParsePasswordHash(cfg.AuthPasswordHash)
	if err != nil {
		t.Fatalf("ParsePasswordHash() error = %v", err)
	}
	if !passwordHash.Verify(password) {
		t.Fatalf("generated password hash did not verify password from file")
	}
	if strings.Contains(cfg.AuthPasswordHash, password) {
		t.Fatalf("AuthPasswordHash contains plaintext password")
	}
}

func TestLoadRejectsAuthPasswordHashAndFile(t *testing.T) {
	t.Parallel()

	passwordFile := filepath.Join(t.TempDir(), "admin-password")
	if err := os.WriteFile(passwordFile, []byte("correct horse battery staple"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(env(map[string]string{
		EnvAuthMode:         AuthModeLogin,
		EnvAuthUsername:     "avery",
		EnvAuthPasswordHash: testPasswordHash(t),
		EnvAuthPasswordFile: passwordFile,
	}))
	if err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("Load() error = %v, want mutual exclusion error", err)
	}
}

func TestLoadRejectsWeakAuthPasswordFile(t *testing.T) {
	t.Parallel()

	passwordFile := filepath.Join(t.TempDir(), "admin-password")
	if err := os.WriteFile(passwordFile, []byte("too short\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(env(map[string]string{
		EnvAuthMode:         AuthModeLogin,
		EnvAuthUsername:     "avery",
		EnvAuthPasswordFile: passwordFile,
	}))
	if err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), EnvAuthPasswordFile) {
		t.Fatalf("Load() error = %v, want %s context", err, EnvAuthPasswordFile)
	}
}

func TestLoadParsesAuthMode(t *testing.T) {
	t.Parallel()

	for name, values := range map[string]map[string]string{
		"basic": {
			EnvAuthMode:         AuthModeBasic,
			EnvAuthUsername:     "avery",
			EnvAuthPasswordHash: testPasswordHash(t),
		},
		"login": {
			EnvAuthMode:         AuthModeLogin,
			EnvAuthUsername:     "avery",
			EnvAuthPasswordHash: testPasswordHash(t),
		},
	} {
		name := name
		values := values
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := Load(env(values))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.AuthMode != values[EnvAuthMode] {
				t.Fatalf("AuthMode = %q, want %q", cfg.AuthMode, values[EnvAuthMode])
			}
		})
	}
}

func TestLoadAcceptsLegacyPBKDF2AuthPasswordHash(t *testing.T) {
	t.Parallel()

	passwordHash, err := auth.HashPBKDF2Password("correct horse battery staple", []byte("0123456789abcdef"), auth.DefaultIterations)
	if err != nil {
		t.Fatalf("HashPBKDF2Password() error = %v", err)
	}
	cfg, err := Load(env(map[string]string{
		EnvAuthMode:         AuthModeBasic,
		EnvAuthUsername:     "avery",
		EnvAuthPasswordHash: passwordHash,
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AuthPasswordHash != passwordHash {
		t.Fatalf("AuthPasswordHash = %q, want legacy PBKDF2 hash", cfg.AuthPasswordHash)
	}
}

func TestLoadParsesAuthSessionConfig(t *testing.T) {
	t.Parallel()

	cfg, err := Load(env(map[string]string{
		EnvSessionIdleTimeout:     "45m",
		EnvSessionAbsoluteTimeout: "45d",
		EnvAuthTrustProxyHeaders:  "true",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SessionIdleTimeout != 45*time.Minute {
		t.Fatalf("SessionIdleTimeout = %s, want 45m", cfg.SessionIdleTimeout)
	}
	if cfg.SessionAbsoluteTimeout != 45*24*time.Hour {
		t.Fatalf("SessionAbsoluteTimeout = %s, want 45d", cfg.SessionAbsoluteTimeout)
	}
	if !cfg.AuthTrustProxyHeaders {
		t.Fatalf("AuthTrustProxyHeaders = false, want true")
	}
}

func TestLoadRejectsPartialAuthEnv(t *testing.T) {
	t.Parallel()

	for name, values := range map[string]map[string]string{
		"username only defaults to basic": {EnvAuthUsername: "avery"},
		"hash only defaults to basic":     {EnvAuthPasswordHash: testPasswordHash(t)},
		"file only defaults to basic":     {EnvAuthPasswordFile: "/tmp/jobhunt-os-password"},
		"login username only": {
			EnvAuthMode:     AuthModeLogin,
			EnvAuthUsername: "avery",
		},
		"login hash only": {
			EnvAuthMode:         AuthModeLogin,
			EnvAuthPasswordHash: testPasswordHash(t),
		},
		"basic username only": {
			EnvAuthMode:     AuthModeBasic,
			EnvAuthUsername: "avery",
		},
		"basic hash only": {
			EnvAuthMode:         AuthModeBasic,
			EnvAuthPasswordHash: testPasswordHash(t),
		},
		"basic file only": {
			EnvAuthMode:         AuthModeBasic,
			EnvAuthPasswordFile: "/tmp/jobhunt-os-password",
		},
	} {
		name := name
		values := values
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Load(env(values)); err == nil {
				t.Fatalf("Load() error = nil, want error")
			}
		})
	}
}

func TestLoadRejectsInvalidAuthPasswordHash(t *testing.T) {
	t.Parallel()

	for _, authMode := range []string{AuthModeBasic, AuthModeLogin} {
		authMode := authMode
		t.Run(authMode, func(t *testing.T) {
			t.Parallel()

			if _, err := Load(env(map[string]string{
				EnvAuthMode:         authMode,
				EnvAuthUsername:     "avery",
				EnvAuthPasswordHash: "not-a-password-hash",
			})); err == nil {
				t.Fatalf("Load() error = nil, want error")
			}
		})
	}
}

func TestLoadRejectsInvalidAuthMode(t *testing.T) {
	t.Parallel()

	if _, err := Load(env(map[string]string{EnvAuthMode: "sometimes"})); err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
}

func TestLoadAllowsLoopbackNoAuth(t *testing.T) {
	t.Parallel()

	cfg, err := Load(env(map[string]string{
		EnvAddr:     "localhost:9090",
		EnvAuthMode: AuthModeDisabled,
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AuthMode != AuthModeDisabled {
		t.Fatalf("AuthMode = %q, want %q", cfg.AuthMode, AuthModeDisabled)
	}
}

func TestLoadRejectsNonLoopbackNoAuth(t *testing.T) {
	t.Parallel()

	if _, err := Load(env(map[string]string{
		EnvAddr:         "0.0.0.0:8080",
		EnvAllowNetwork: "true",
		EnvAuthMode:     AuthModeDisabled,
	})); err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
}

func TestLoadAllowsNonLoopbackNoAuthWithEscapeHatch(t *testing.T) {
	t.Parallel()

	cfg, err := Load(env(map[string]string{
		EnvAddr:                "0.0.0.0:8080",
		EnvAllowNetwork:        "true",
		EnvAuthMode:            AuthModeDisabled,
		EnvAllowInsecureNoAuth: "true",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.AllowInsecureNoAuth {
		t.Fatalf("AllowInsecureNoAuth = false, want true")
	}
}

func TestLoadRejectsInvalidAuthBooleansAndDurations(t *testing.T) {
	t.Parallel()

	for name, values := range map[string]map[string]string{
		"allow insecure": {EnvAllowInsecureNoAuth: "sometimes"},
		"trust proxy":    {EnvAuthTrustProxyHeaders: "sometimes"},
		"idle timeout":   {EnvSessionIdleTimeout: "later"},
		"absolute":       {EnvSessionAbsoluteTimeout: "0s"},
	} {
		name := name
		values := values
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Load(env(values)); err == nil {
				t.Fatalf("Load() error = nil, want error")
			}
		})
	}
}

func TestLoadUsesExplicitDataDir(t *testing.T) {
	t.Parallel()

	cfg, err := load(env(map[string]string{
		EnvDataDir: "/tmp/jobhunt-os-data/../jobhunt-os",
	}), loadOptions{
		goos:    "linux",
		homeDir: "/home/jordan",
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	want := filepath.Clean("/tmp/jobhunt-os")
	if cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
}

func TestDefaultDataDirDarwin(t *testing.T) {
	t.Parallel()

	cfg, err := load(env(nil), loadOptions{
		goos:    "darwin",
		homeDir: "/Users/jordan",
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	want := filepath.Join("/Users/jordan", "Library", "Application Support", "jobhunt-os")
	if cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
}

func TestDefaultDataDirLinuxWithXDGDataHome(t *testing.T) {
	t.Parallel()

	cfg, err := load(env(map[string]string{
		envXDGDataHome: "/var/lib/user-data",
	}), loadOptions{
		goos:    "linux",
		homeDir: "/home/jordan",
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	want := filepath.Join("/var/lib/user-data", "jobhunt-os")
	if cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
}

func TestDefaultDataDirLinuxWithoutXDGDataHome(t *testing.T) {
	t.Parallel()

	cfg, err := load(env(nil), loadOptions{
		goos:    "linux",
		homeDir: "/home/jordan",
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	want := filepath.Join("/home/jordan", ".local", "share", "jobhunt-os")
	if cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
}

func TestDefaultDataDirWindows(t *testing.T) {
	t.Parallel()

	cfg, err := load(env(map[string]string{
		envAppData: `C:\Users\Jordan\AppData\Roaming`,
	}), loadOptions{
		goos:    "windows",
		homeDir: `C:\Users\Jordan`,
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	want := `C:\Users\Jordan\AppData\Roaming\jobhunt-os`
	if cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
}

func TestDataDirExpandsHomePrefix(t *testing.T) {
	t.Parallel()

	cfg, err := load(env(map[string]string{
		EnvDataDir: "~/Library/Application Support/jobhunt-os",
	}), loadOptions{
		goos:    "darwin",
		homeDir: "/Users/jordan",
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}

	want := filepath.Join("/Users/jordan", "Library", "Application Support", "jobhunt-os")
	if cfg.DataDir != want {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, want)
	}
}

func TestLoadRejectsInvalidAllowNetwork(t *testing.T) {
	t.Parallel()

	if _, err := Load(env(map[string]string{EnvAllowNetwork: "sometimes"})); err == nil {
		t.Fatalf("Load() error = nil, want error")
	}
}

func TestValidateAddrAllowsLoopback(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{
		"127.0.0.1:8080",
		"localhost:8080",
		"[::1]:8080",
	} {
		addr := addr
		t.Run(addr, func(t *testing.T) {
			t.Parallel()

			if err := ValidateAddr(addr, false); err != nil {
				t.Fatalf("ValidateAddr(%q, false) error = %v", addr, err)
			}
		})
	}
}

func TestValidateAddrRejectsNonLoopbackByDefault(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{
		"0.0.0.0:8080",
		":8080",
		"192.168.1.25:8080",
		"example.com:8080",
		"[::]:8080",
	} {
		addr := addr
		t.Run(addr, func(t *testing.T) {
			t.Parallel()

			if err := ValidateAddr(addr, false); err == nil {
				t.Fatalf("ValidateAddr(%q, false) error = nil, want error", addr)
			}
		})
	}
}

func TestValidateAddrAllowsNonLoopbackWithEscapeHatch(t *testing.T) {
	t.Parallel()

	if err := ValidateAddr("0.0.0.0:8080", true); err != nil {
		t.Fatalf("ValidateAddr(0.0.0.0:8080, true) error = %v", err)
	}
}

func TestValidateAddrRejectsMissingPort(t *testing.T) {
	t.Parallel()

	if err := ValidateAddr("127.0.0.1", false); err == nil {
		t.Fatalf("ValidateAddr() error = nil, want error")
	}
}

func env(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func testPasswordHash(t *testing.T) string {
	t.Helper()

	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	return hash
}
