package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/auth"
)

const (
	EnvAddr                   = "JOBHUNT_ADDR"
	EnvAllowNetwork           = "JOBHUNT_ALLOW_NETWORK"
	EnvDataDir                = "JOBHUNT_DATA_DIR"
	EnvAuthMode               = "JOBHUNT_AUTH_MODE"
	EnvAuthUsername           = "JOBHUNT_AUTH_USERNAME"
	EnvAuthPasswordHash       = "JOBHUNT_AUTH_PASSWORD_HASH"
	EnvAllowInsecureNoAuth    = "JOBHUNT_ALLOW_INSECURE_NO_AUTH"
	EnvSessionIdleTimeout     = "JOBHUNT_SESSION_IDLE_TIMEOUT"
	EnvSessionAbsoluteTimeout = "JOBHUNT_SESSION_ABSOLUTE_TIMEOUT"
	EnvAuthTrustProxyHeaders  = "JOBHUNT_AUTH_TRUST_PROXY_HEADERS"
	EnvSecureCookies          = "JOBHUNT_SECURE_COOKIES"

	DefaultAddr = "127.0.0.1:8080"
)

const (
	AuthModeDisabled = "disabled"
	AuthModeLogin    = "login"
	AuthModeBasic    = "basic"
)

const (
	envAppData     = "APPDATA"
	envXDGDataHome = "XDG_DATA_HOME"
)

type Config struct {
	Addr                   string
	AllowNetwork           bool
	DataDir                string
	AuthMode               string
	AuthUsername           string
	AuthPasswordHash       string
	AllowInsecureNoAuth    bool
	SessionIdleTimeout     time.Duration
	SessionAbsoluteTimeout time.Duration
	AuthTrustProxyHeaders  bool
	SecureCookies          bool
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
	IdleTimeout            time.Duration
}

func Load(getenv func(string) string) (Config, error) {
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		homeDir = ""
	}

	return load(getenv, loadOptions{
		goos:    runtime.GOOS,
		homeDir: homeDir,
	})
}

type loadOptions struct {
	goos    string
	homeDir string
}

func load(getenv func(string) string, opts loadOptions) (Config, error) {
	dataDir, err := dataDir(getenv, opts.goos, opts.homeDir)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Addr:                   strings.TrimSpace(getenv(EnvAddr)),
		DataDir:                dataDir,
		SessionIdleTimeout:     12 * time.Hour,
		SessionAbsoluteTimeout: 30 * 24 * time.Hour,
		ReadTimeout:            5 * time.Second,
		WriteTimeout:           10 * time.Second,
		IdleTimeout:            2 * time.Minute,
	}
	if cfg.Addr == "" {
		cfg.Addr = DefaultAddr
	}

	if cfg.AllowNetwork, err = parseOptionalBool(getenv, EnvAllowNetwork); err != nil {
		return Config{}, err
	}
	if cfg.AllowInsecureNoAuth, err = parseOptionalBool(getenv, EnvAllowInsecureNoAuth); err != nil {
		return Config{}, err
	}

	if err := ValidateAddr(cfg.Addr, cfg.AllowNetwork); err != nil {
		return Config{}, err
	}

	if cfg.SecureCookies, err = parseOptionalBool(getenv, EnvSecureCookies); err != nil {
		return Config{}, err
	}
	if cfg.AuthTrustProxyHeaders, err = parseOptionalBool(getenv, EnvAuthTrustProxyHeaders); err != nil {
		return Config{}, err
	}
	if cfg.SessionIdleTimeout, err = parseOptionalDuration(getenv, EnvSessionIdleTimeout, cfg.SessionIdleTimeout); err != nil {
		return Config{}, err
	}
	if cfg.SessionAbsoluteTimeout, err = parseOptionalDuration(getenv, EnvSessionAbsoluteTimeout, cfg.SessionAbsoluteTimeout); err != nil {
		return Config{}, err
	}

	authUsername := strings.TrimSpace(getenv(EnvAuthUsername))
	authPasswordHash := strings.TrimSpace(getenv(EnvAuthPasswordHash))
	authMode, err := parseAuthMode(getenv, authUsername, authPasswordHash)
	if err != nil {
		return Config{}, err
	}
	cfg.AuthMode = authMode

	if err := validateAuthConfig(cfg.AuthMode, cfg.Addr, cfg.AllowInsecureNoAuth, authUsername, authPasswordHash); err != nil {
		return Config{}, err
	}
	if cfg.AuthMode != AuthModeDisabled {
		cfg.AuthUsername = authUsername
		cfg.AuthPasswordHash = authPasswordHash
	}

	return cfg, nil
}

func parseOptionalBool(getenv func(string) string, key string) (bool, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func parseOptionalDuration(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(getenv(key))
	if value == "" {
		return fallback, nil
	}

	duration, err := parseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}
	return duration, nil
}

func parseDuration(value string) (time.Duration, error) {
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(value)
}

func parseAuthMode(getenv func(string) string, authUsername string, authPasswordHash string) (string, error) {
	authMode := strings.TrimSpace(strings.ToLower(getenv(EnvAuthMode)))
	if authMode == "" {
		if authUsername != "" || authPasswordHash != "" {
			return AuthModeBasic, nil
		}
		return AuthModeDisabled, nil
	}

	switch authMode {
	case AuthModeDisabled, AuthModeLogin, AuthModeBasic:
		return authMode, nil
	default:
		return "", fmt.Errorf("%s must be one of %s, %s, or %s", EnvAuthMode, AuthModeDisabled, AuthModeLogin, AuthModeBasic)
	}
}

func validateAuthConfig(authMode string, addr string, allowInsecureNoAuth bool, authUsername string, authPasswordHash string) error {
	switch authMode {
	case AuthModeDisabled:
		if authUsername != "" || authPasswordHash != "" {
			return fmt.Errorf("%s and %s must be empty when %s=%s", EnvAuthUsername, EnvAuthPasswordHash, EnvAuthMode, AuthModeDisabled)
		}
		if !allowInsecureNoAuth && !isLoopbackAddr(addr) {
			return fmt.Errorf("%s=%s is refused for non-loopback %s=%q; set %s=true only if this deployment is protected elsewhere", EnvAuthMode, AuthModeDisabled, EnvAddr, addr, EnvAllowInsecureNoAuth)
		}
		return nil
	case AuthModeLogin, AuthModeBasic:
		if authUsername == "" {
			return fmt.Errorf("%s is required when %s=%s", EnvAuthUsername, EnvAuthMode, authMode)
		}
		if authPasswordHash == "" {
			return fmt.Errorf("%s is required when %s=%s", EnvAuthPasswordHash, EnvAuthMode, authMode)
		}
		if _, err := auth.ParsePasswordHash(authPasswordHash); err != nil {
			return fmt.Errorf("%s must be a supported password hash: %w", EnvAuthPasswordHash, err)
		}
		return nil
	default:
		return fmt.Errorf("%s must be one of %s, %s, or %s", EnvAuthMode, AuthModeDisabled, AuthModeLogin, AuthModeBasic)
	}
}

func dataDir(getenv func(string) string, targetGOOS string, homeDir string) (string, error) {
	if explicit := strings.TrimSpace(getenv(EnvDataDir)); explicit != "" {
		expanded, err := expandHome(explicit, homeDir)
		if err != nil {
			return "", err
		}
		return cleanDataPath(expanded), nil
	}

	switch targetGOOS {
	case "darwin":
		if strings.TrimSpace(homeDir) == "" {
			return "", fmt.Errorf("home directory is required to default %s", EnvDataDir)
		}
		return cleanDataPath(filepath.Join(homeDir, "Library", "Application Support", "jobhunt-os")), nil
	case "windows":
		appData := strings.TrimSpace(getenv(envAppData))
		if appData == "" {
			if strings.TrimSpace(homeDir) == "" {
				return "", fmt.Errorf("%s or home directory is required to default %s", envAppData, EnvDataDir)
			}
			appData = windowsJoin(homeDir, "AppData", "Roaming")
		}
		return cleanDataPath(windowsJoin(appData, "jobhunt-os")), nil
	default:
		if xdgDataHome := strings.TrimSpace(getenv(envXDGDataHome)); xdgDataHome != "" {
			expanded, err := expandHome(xdgDataHome, homeDir)
			if err != nil {
				return "", err
			}
			return cleanDataPath(filepath.Join(expanded, "jobhunt-os")), nil
		}
		if strings.TrimSpace(homeDir) == "" {
			return "", fmt.Errorf("home directory is required to default %s", EnvDataDir)
		}
		return cleanDataPath(filepath.Join(homeDir, ".local", "share", "jobhunt-os")), nil
	}
}

func expandHome(path string, homeDir string) (string, error) {
	if path == "~" {
		if strings.TrimSpace(homeDir) == "" {
			return "", fmt.Errorf("home directory is required to expand %s", EnvDataDir)
		}
		return homeDir, nil
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if strings.TrimSpace(homeDir) == "" {
			return "", fmt.Errorf("home directory is required to expand %s", EnvDataDir)
		}
		return filepath.Join(homeDir, path[2:]), nil
	}
	return path, nil
}

func cleanDataPath(path string) string {
	return filepath.Clean(path)
}

func windowsJoin(base string, elems ...string) string {
	parts := make([]string, 0, len(elems)+1)
	parts = append(parts, strings.TrimRight(base, `\/`))
	for _, elem := range elems {
		trimmed := strings.Trim(elem, `\/`)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, `\`)
}

func ValidateAddr(addr string, allowNetwork bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("%s must be host:port: %w", EnvAddr, err)
	}

	if allowNetwork {
		return nil
	}

	if isLoopbackHost(host) {
		return nil
	}

	return fmt.Errorf("%s=%q is not loopback; set %s=true to allow network binding", EnvAddr, addr, EnvAllowNetwork)
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return isLoopbackHost(host)
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if host == "" {
		return false
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
