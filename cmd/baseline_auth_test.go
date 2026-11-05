package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/credstore"
)

func TestDoctorCanAutoLoginFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")

	out, err := runIn(t, dir, "doctor", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	res, _ := env["result"].(map[string]any)
	if res == nil {
		t.Fatalf("missing result: %s", out)
	}
	if v, ok := res["can_auto_login"].(bool); !ok || v {
		t.Fatalf("can_auto_login want false got %v out=%s", res["can_auto_login"], out)
	}
	if v, ok := res["password_stored"].(bool); !ok || v {
		t.Fatalf("password_stored want false got %v", res["password_stored"])
	}
	if strings.Contains(out, "secret") || strings.Contains(out, "password_protected") {
		t.Fatalf("doctor leaked secret material: %s", out)
	}

	if err := credstore.Save(dir, credstore.Cred{Username: "alice", Password: "store-pass-secret", AutoLogin: true}); err != nil {
		t.Fatal(err)
	}
	out, err = runIn(t, dir, "doctor", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	res, _ = env["result"].(map[string]any)
	if v, ok := res["can_auto_login"].(bool); !ok || !v {
		t.Fatalf("can_auto_login want true with remembered+auto_login pref: %v out=%s", res["can_auto_login"], out)
	}
	if v, ok := res["password_stored"].(bool); !ok || !v {
		t.Fatalf("password_stored want true: %v", res["password_stored"])
	}
	if res["username"] != "alice" {
		t.Fatalf("username=%v", res["username"])
	}
	if strings.Contains(out, "store-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}

	// env pair alone (no store) + explicit --auto-login
	dir2 := t.TempDir()
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "env-user")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "env-pass-secret")
	out, err = runIn(t, dir2, "doctor", "--base-url", srv.URL, "--auto-login")
	if err != nil {
		t.Fatal(err, out)
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	res, _ = env["result"].(map[string]any)
	if v, ok := res["can_auto_login"].(bool); !ok || !v {
		t.Fatalf("can_auto_login want true with env pair + --auto-login: %v out=%s", res["can_auto_login"], out)
	}
	if strings.Contains(out, "env-pass-secret") {
		t.Fatalf("env password leaked: %s", out)
	}
}

func TestLogoutKeepsLoginCredUnlessForget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := credstore.Save(dir, credstore.Cred{Username: "u", Password: "keep-me-secret", AutoLogin: true}); err != nil {
		t.Fatal(err)
	}
	// seed a cookie jar so logout has something to clear locally
	cookiePath := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookiePath, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runIn(t, dir, "auth", "logout", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "login.cred")); err != nil {
		t.Fatal("login.cred should remain after default logout", err)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	res, _ := env["result"].(map[string]any)
	if v, _ := res["forgot_password"].(bool); v {
		t.Fatalf("forgot_password should be false: %s", out)
	}
	if strings.Contains(out, "keep-me-secret") {
		t.Fatalf("password leaked: %s", out)
	}

	out, err = runIn(t, dir, "auth", "logout", "--forget", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "login.cred")); !os.IsNotExist(err) {
		t.Fatal("login.cred should be cleared with --forget", err)
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	res, _ = env["result"].(map[string]any)
	if v, _ := res["forgot_password"].(bool); !v {
		t.Fatalf("forgot_password should be true: %s", out)
	}
	if v, _ := res["cleared_login_cred"].(bool); !v {
		t.Fatalf("cleared_login_cred should be true: %s", out)
	}

	// --clear-password alias
	if err := credstore.Save(dir, credstore.Cred{Username: "u2", Password: "alias-secret", AutoLogin: true}); err != nil {
		t.Fatal(err)
	}
	out, err = runIn(t, dir, "auth", "logout", "--clear-password", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "login.cred")); !os.IsNotExist(err) {
		t.Fatal("login.cred should be cleared with --clear-password", err)
	}
}
