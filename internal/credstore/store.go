// Package credstore stores login username + encrypted password for opt-in auto-login.
// Passwords are never written to config.json. Plaintext is never logged by this package.
package credstore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	fileName = "login.cred"
	version  = 1
)

// Cred is the decrypted in-memory credential. Password must not be logged.
type Cred struct {
	Username  string
	Password  string
	AutoLogin bool
}

type diskFile struct {
	Version           int    `json:"version"`
	Username          string `json:"username"`
	PasswordProtected string `json:"password_protected"`
	Protect           string `json:"protect"`
	AutoLogin         bool   `json:"auto_login"`
}

// Path returns the encrypted credential file under configDir.
func Path(configDir string) string {
	return filepath.Join(configDir, fileName)
}

// DirFromConfigPath returns the directory that holds config.json (and login.cred).
func DirFromConfigPath(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Dir(configPath)
}

// Save encrypts the password and writes login.cred with mode 0600.
func Save(configDir string, c Cred) error {
	if strings.TrimSpace(configDir) == "" {
		return errors.New("credstore: config dir is empty")
	}
	user := strings.TrimSpace(c.Username)
	if user == "" {
		return errors.New("credstore: username is required")
	}
	if c.Password == "" {
		return errors.New("credstore: password is required")
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	blob, scheme, err := protect([]byte(c.Password), configDir)
	if err != nil {
		return err
	}
	df := diskFile{
		Version:           version,
		Username:          user,
		PasswordProtected: base64.StdEncoding.EncodeToString(blob),
		Protect:           scheme,
		AutoLogin:         c.AutoLogin,
	}
	b, err := json.MarshalIndent(df, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	path := Path(configDir)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

// Load decrypts login.cred. Missing file returns ok=false and no error.
func Load(configDir string) (Cred, bool, error) {
	path := Path(configDir)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Cred{}, false, nil
		}
		return Cred{}, false, err
	}
	var df diskFile
	if err := json.Unmarshal(b, &df); err != nil {
		return Cred{}, true, fmt.Errorf("credstore: parse %s: %w", path, err)
	}
	if df.Version != 0 && df.Version != version {
		return Cred{}, true, fmt.Errorf("credstore: unsupported version %d", df.Version)
	}
	raw, err := base64.StdEncoding.DecodeString(df.PasswordProtected)
	if err != nil {
		return Cred{}, true, fmt.Errorf("credstore: corrupt ciphertext: %w", err)
	}
	plain, err := unprotect(raw, df.Protect, configDir)
	if err != nil {
		return Cred{}, true, err
	}
	return Cred{
		Username:  strings.TrimSpace(df.Username),
		Password:  string(plain),
		AutoLogin: df.AutoLogin,
	}, true, nil
}

// Clear removes login.cred (and the local AES key file when present). Missing is OK.
func Clear(configDir string) error {
	if strings.TrimSpace(configDir) == "" {
		return nil
	}
	_ = os.Remove(localKeyPath(configDir))
	err := os.Remove(Path(configDir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// PeekUsernameAutoLogin reads username and auto_login without decrypting the password.
func PeekUsernameAutoLogin(configDir string) (username string, autoLogin bool, ok bool) {
	df, exists, err := peek(configDir)
	if err != nil || !exists {
		return "", false, false
	}
	return strings.TrimSpace(df.Username), df.AutoLogin, true
}

func peek(configDir string) (diskFile, bool, error) {
	b, err := os.ReadFile(Path(configDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return diskFile{}, false, nil
		}
		return diskFile{}, false, err
	}
	var df diskFile
	if err := json.Unmarshal(b, &df); err != nil {
		return diskFile{}, true, err
	}
	return df, true, nil
}
