package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/credstore"
)

func TestAutoLoginDefaultsOnEnvPairWithoutFlag(t *testing.T) {
	var hits atomic.Int32
	var loginHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), "env-alice") || !strings.Contains(string(b), "env-pass-secret") {
				t.Errorf("login body %s", b)
			}
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "from-env", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"logs":[]}`)
		case "/getAppNames":
			n := hits.Add(1)
			if n == 1 {
				_, _ = io.WriteString(w, `{"code":401,"logs":[]}`)
				return
			}
			if !strings.Contains(r.Header.Get("Cookie"), "JSESSIONID=from-env") {
				t.Errorf("replay missing cookie: %q", r.Header.Get("Cookie"))
			}
			_, _ = io.WriteString(w, `["demo"]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "env-alice")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "env-pass-secret")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")

	out, err := runIn(t, dir, "query", "apps", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "env-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
	if hits.Load() != 2 || loginHits.Load() != 1 {
		t.Fatalf("hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
}

func TestNoAutoLoginFlagDisablesRememberedCred(t *testing.T) {
	hits, loginHits, srv := unauthorizedAppsServer(t)
	defer srv.Close()

	dir := t.TempDir()
	if err := credstore.Save(dir, credstore.Cred{Username: "store-user", Password: "store-pass-secret", AutoLogin: true}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")

	out, err := runIn(t, dir, "query", "apps", "--base-url", srv.URL, "--no-auto-login")
	if err == nil {
		t.Fatal("expected 401 without silent re-login", out)
	}
	if hits.Load() != 1 || loginHits.Load() != 0 {
		t.Fatalf("hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
	if strings.Contains(out, "store-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
}

func TestAutoLoginEnvZeroDisablesRememberedCred(t *testing.T) {
	hits, loginHits, srv := unauthorizedAppsServer(t)
	defer srv.Close()

	dir := t.TempDir()
	if err := credstore.Save(dir, credstore.Cred{Username: "store-user", Password: "store-pass-secret", AutoLogin: true}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "0")

	out, err := runIn(t, dir, "query", "apps", "--base-url", srv.URL)
	if err == nil {
		t.Fatal("expected 401 with PLUMELOG_AUTO_LOGIN=0", out)
	}
	if hits.Load() != 1 || loginHits.Load() != 0 {
		t.Fatalf("hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
	if strings.Contains(out, "store-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
}

func TestDoctorCanAutoLoginEnvPairWithoutFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "env-user")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "env-pass-secret")
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
	if v, ok := res["can_auto_login"].(bool); !ok || !v {
		t.Fatalf("can_auto_login want true with env pair by default: %v out=%s", res["can_auto_login"], out)
	}
	if strings.Contains(out, "env-pass-secret") {
		t.Fatalf("env password leaked: %s", out)
	}

	out, err = runIn(t, dir, "doctor", "--base-url", srv.URL, "--no-auto-login")
	if err != nil {
		t.Fatal(err, out)
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	res, _ = env["result"].(map[string]any)
	if v, ok := res["can_auto_login"].(bool); !ok || v {
		t.Fatalf("can_auto_login want false with --no-auto-login: %v out=%s", res["can_auto_login"], out)
	}
}

func unauthorizedAppsServer(t *testing.T) (*atomic.Int32, *atomic.Int32, *httptest.Server) {
	t.Helper()
	var hits atomic.Int32
	var loginHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "should-not-login", Path: "/"})
			_, _ = io.WriteString(w, `{"logs":[]}`)
		case "/getAppNames":
			hits.Add(1)
			_, _ = io.WriteString(w, `{"code":401,"logs":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	return &hits, &loginHits, srv
}

func TestAutoLoginFlagOverridesEnvZero(t *testing.T) {
	var hits atomic.Int32
	var loginHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "from-flag", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"logs":[]}`)
		case "/getAppNames":
			n := hits.Add(1)
			if n == 1 {
				_, _ = io.WriteString(w, `{"code":401,"logs":[]}`)
				return
			}
			if !strings.Contains(r.Header.Get("Cookie"), "JSESSIONID=from-flag") {
				t.Errorf("replay missing cookie: %q", r.Header.Get("Cookie"))
			}
			_, _ = io.WriteString(w, `["demo"]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "flag-alice")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "flag-pass-secret")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "0")

	out, err := runIn(t, dir, "query", "apps", "--base-url", srv.URL, "--auto-login")
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "flag-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
	if hits.Load() != 2 || loginHits.Load() != 1 {
		t.Fatalf("hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
}

func TestDoctorSilentReloginOn401(t *testing.T) {
	var runHits atomic.Int32
	var loginHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "doctor-session", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/getRunModel", "/getQueueCounts":
			if r.URL.Path == "/getRunModel" {
				n := runHits.Add(1)
				if n == 1 {
					_, _ = io.WriteString(w, `{"code":401,"msg":"unauth"}`)
					return
				}
			}
			if !strings.Contains(r.Header.Get("Cookie"), "JSESSIONID=doctor-session") {
				t.Errorf("%s missing cookie: %q", r.URL.Path, r.Header.Get("Cookie"))
			}
			_, _ = io.WriteString(w, `{"code":0,"msg":"ok"}`)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "doc-user")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "doc-pass-secret")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")

	out, err := runIn(t, dir, "doctor", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "doc-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
	if runHits.Load() != 2 || loginHits.Load() != 1 {
		t.Fatalf("runHits=%d login=%d out=%s", runHits.Load(), loginHits.Load(), out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	if env["ok"] != true {
		t.Fatalf("doctor should be healthy after silent re-login: %s", out)
	}

	runHits.Store(0)
	loginHits.Store(0)
	out, err = runIn(t, dir, "doctor", "--base-url", srv.URL, "--no-auto-login")
	if err == nil {
		t.Fatal("expected doctor failure without silent re-login", out)
	}
	if runHits.Load() != 1 || loginHits.Load() != 0 {
		t.Fatalf("no-auto-login runHits=%d login=%d out=%s", runHits.Load(), loginHits.Load(), out)
	}
	if strings.Contains(out, "doc-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
}

func TestAuthStatusProbeSilentReloginOn401(t *testing.T) {
	var hits atomic.Int32
	var loginHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "status-session", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"code":0}`)
		case "/getRunModel":
			n := hits.Add(1)
			if n == 1 {
				_, _ = io.WriteString(w, `{"code":401}`)
				return
			}
			if !strings.Contains(r.Header.Get("Cookie"), "JSESSIONID=status-session") {
				t.Errorf("replay missing cookie: %q", r.Header.Get("Cookie"))
			}
			_, _ = io.WriteString(w, `{"code":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	t.Setenv("PLUMELOG_LOGIN_USERNAME", "st-user")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "st-pass-secret")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")

	out, err := runIn(t, dir, "auth", "status", "--probe", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if hits.Load() != 2 || loginHits.Load() != 1 {
		t.Fatalf("hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
	if strings.Contains(out, "st-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	res, _ := env["result"].(map[string]any)
	if res["authenticated"] != true {
		t.Fatalf("authenticated want true after silent re-login: %s", out)
	}

	hits.Store(0)
	loginHits.Store(0)
	out, err = runIn(t, dir, "whoami", "--probe", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if hits.Load() != 2 || loginHits.Load() != 1 {
		t.Fatalf("whoami hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
}
