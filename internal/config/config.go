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
)

const (
	EnvAddr         = "JOBHUNT_ADDR"
	EnvAllowNetwork = "JOBHUNT_ALLOW_NETWORK"
	EnvDataDir      = "JOBHUNT_DATA_DIR"

	DefaultAddr = "127.0.0.1:8080"
)

const (
	envAppData     = "APPDATA"
	envXDGDataHome = "XDG_DATA_HOME"
)

type Config struct {
	Addr         string
	AllowNetwork bool
	DataDir      string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
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
		Addr:         strings.TrimSpace(getenv(EnvAddr)),
		DataDir:      dataDir,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  2 * time.Minute,
	}
	if cfg.Addr == "" {
		cfg.Addr = DefaultAddr
	}

	allowNetwork := strings.TrimSpace(getenv(EnvAllowNetwork))
	if allowNetwork != "" {
		parsed, err := strconv.ParseBool(allowNetwork)
		if err != nil {
			return Config{}, fmt.Errorf("%s must be a boolean: %w", EnvAllowNetwork, err)
		}
		cfg.AllowNetwork = parsed
	}

	if err := ValidateAddr(cfg.Addr, cfg.AllowNetwork); err != nil {
		return Config{}, err
	}

	return cfg, nil
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
