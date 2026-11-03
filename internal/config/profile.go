// Named profiles live under profiles/<name>/; the default profile keeps the
// legacy flat layout (config.json + cookies.txt) for v0.1.4 compatibility.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// DefaultProfile is the legacy single-instance profile name.
	DefaultProfile = "default"

	profilesMetaFile = "profiles.json"
	profilesDirName  = "profiles"
)

var profileNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// Meta is the on-disk profiles.json (current profile pointer only).
type Meta struct {
	Current string `json:"current"`
}

// ValidateProfileName rejects empty, path-like, or otherwise unsafe names.
func ValidateProfileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("profile name is empty")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid profile name %q", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid profile name %q", name)
	}
	if !profileNameRE.MatchString(name) {
		return fmt.Errorf("invalid profile name %q (want letters, digits, _ or -, max 64)", name)
	}
	return nil
}

// MetaPath returns profiles.json under the config directory.
func MetaPath(configDir string) string {
	return filepath.Join(configDir, profilesMetaFile)
}

// LoadMeta reads profiles.json. Missing file yields Current=default.
func LoadMeta(configDir string) (Meta, error) {
	b, err := os.ReadFile(MetaPath(configDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Meta{Current: DefaultProfile}, nil
		}
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("parse %s: %w", MetaPath(configDir), err)
	}
	if strings.TrimSpace(m.Current) == "" {
		m.Current = DefaultProfile
	}
	if err := ValidateProfileName(m.Current); err != nil {
		return Meta{}, fmt.Errorf("profiles.json current: %w", err)
	}
	return m, nil
}

// SaveMeta writes profiles.json with mode 0600.
func SaveMeta(configDir string, m Meta) error {
	if strings.TrimSpace(m.Current) == "" {
		m.Current = DefaultProfile
	}
	if err := ValidateProfileName(m.Current); err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path := MetaPath(configDir)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// CurrentProfile returns the active profile name from profiles.json (or default).
func CurrentProfile(configDir string) (string, error) {
	m, err := LoadMeta(configDir)
	if err != nil {
		return "", err
	}
	return m.Current, nil
}

// SetCurrent writes the current profile pointer. Creates the profile dir when needed.
func SetCurrent(configDir, name string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	if err := EnsureProfile(configDir, name); err != nil {
		return err
	}
	return SaveMeta(configDir, Meta{Current: name})
}

// PathsForProfile returns config.json and cookies.txt paths for a profile.
// The default profile uses the legacy flat layout for backward compatibility.
func PathsForProfile(configDir, name string) (configPath, cookiePath string, err error) {
	if err := ValidateProfileName(name); err != nil {
		return "", "", err
	}
	if name == DefaultProfile {
		return filepath.Join(configDir, "config.json"), filepath.Join(configDir, "cookies.txt"), nil
	}
	dir := filepath.Join(configDir, profilesDirName, name)
	return filepath.Join(dir, "config.json"), filepath.Join(dir, "cookies.txt"), nil
}

// ProfileDir returns the directory that holds a profile's config, cookies, and login.cred.
func ProfileDir(configDir, name string) (string, error) {
	cfg, _, err := PathsForProfile(configDir, name)
	if err != nil {
		return "", err
	}
	return filepath.Dir(cfg), nil
}

// EnsureProfile creates the profile directory (0700). Default uses the config dir itself.
func EnsureProfile(configDir, name string) error {
	dir, err := ProfileDir(configDir, name)
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o700)
}

// ListProfiles returns sorted profile names: always includes default, plus
// existing profiles/<name>/ directories (and the current pointer if set).
func ListProfiles(configDir string) ([]string, error) {
	set := map[string]struct{}{DefaultProfile: {}}
	m, err := LoadMeta(configDir)
	if err != nil {
		return nil, err
	}
	set[m.Current] = struct{}{}

	entries, err := os.ReadDir(filepath.Join(configDir, profilesDirName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if ValidateProfileName(name) != nil || name == DefaultProfile {
			continue
		}
		set[name] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i] == DefaultProfile {
			return true
		}
		if out[j] == DefaultProfile {
			return false
		}
		return out[i] < out[j]
	})
	return out, nil
}

// ResolveProfileName picks flag > env > profiles.json current > default.
func ResolveProfileName(flagProfile, envProfile, configDir string) (string, error) {
	for _, c := range []string{strings.TrimSpace(flagProfile), strings.TrimSpace(envProfile)} {
		if c == "" {
			continue
		}
		if err := ValidateProfileName(c); err != nil {
			return "", err
		}
		return c, nil
	}
	return CurrentProfile(configDir)
}
