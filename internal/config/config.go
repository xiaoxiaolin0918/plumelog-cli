// Package config loads and stores ~/.config/plumelog/config.json.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// File is the on-disk configuration. Secrets are intentionally absent.
type File struct {
	BaseURL           string  `json:"base_url,omitempty"`
	PathPrefix        *string `json:"path_prefix,omitempty"`
	TimeoutSec        int     `json:"timeout_sec,omitempty"`
	DefaultSize       int     `json:"default_size,omitempty"`
	DefaultTimeWindow string  `json:"default_time_window,omitempty"`
	Format            string  `json:"format,omitempty"`
	CookieJar         string  `json:"cookie_jar,omitempty"`
	LoginUsername     string  `json:"login_username,omitempty"`
}

// Dir is the plumelog config directory.
// Windows: %APPDATA%\plumelog
// Unix: $XDG_CONFIG_HOME/plumelog or ~/.config/plumelog
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return dirFor(runtime.GOOS, os.Getenv("APPDATA"), os.Getenv("XDG_CONFIG_HOME"), home)
}

func dirFor(goos, appdata, xdg, home string) (string, error) {
	if goos == "windows" && appdata != "" {
		return filepath.Join(appdata, "plumelog"), nil
	}
	if xdg != "" {
		return filepath.Join(xdg, "plumelog"), nil
	}
	if home == "" {
		return "", errors.New("home directory is unknown")
	}
	return filepath.Join(home, ".config", "plumelog"), nil
}

// DefaultPaths returns the config file and Netscape cookie jar paths.
func DefaultPaths() (configPath, cookiePath string, err error) {
	dir, err := Dir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "config.json"), filepath.Join(dir, "cookies.txt"), nil
}

// Load reads path. A missing file is not an error; exists is false.
func Load(path string) (File, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, false, nil
		}
		return File{}, false, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, true, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.PathPrefix != nil {
		switch *f.PathPrefix {
		case "", "/plumelog":
		default:
			return File{}, true, fmt.Errorf("config path_prefix %q must be \"\" or \"/plumelog\"", *f.PathPrefix)
		}
	}
	return f, true, nil
}

// Save writes the file with mode 0600, creating the parent directory at 0700.
func Save(path string, f File) error {
	if f.PathPrefix != nil {
		switch *f.PathPrefix {
		case "", "/plumelog":
		default:
			return fmt.Errorf("path_prefix %q must be \"\" or \"/plumelog\"", *f.PathPrefix)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// SecretKey reports keys that must never be stored in config.json.
func SecretKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password", "login_password", "admin_password", "adminpassword", "cookie", "secret", "token":
		return true
	default:
		return false
	}
}

// Keys that config get/set understand.
func Keys() []string {
	return []string{
		"base_url",
		"path_prefix",
		"timeout_sec",
		"default_size",
		"default_time_window",
		"format",
		"cookie_jar",
		"login_username",
	}
}
