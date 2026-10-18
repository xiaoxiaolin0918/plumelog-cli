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

func TestSearchContentWildcardDryRun(t *testing.T) {
	out, err := run(t, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"-q", "OutOfMemoryError",
		"--content-wildcard",
		"--from-time", "1700000000000",
		"--to-time", "1700003600000",
	)
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, `*OutOfMemoryError*`) {
		t.Fatalf("expected wildcard wrap: %s", out)
	}

	out2, err := run(t, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"-q", "java.lang.OutOfMemoryError",
		"--from-time", "1700000000000",
		"--to-time", "1700003600000",
	)
	if err != nil {
		t.Fatal(err, out2)
	}
	if !strings.Contains(out2, `*java.lang.OutOfMemoryError*`) {
		t.Fatalf("expected auto dotted wrap: %s", out2)
	}
}

func TestAutoLoginOn401ThenReplay(t *testing.T) {
	var hits atomic.Int32
	var loginHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			b, _ := io.ReadAll(r.Body)
			if strings.Contains(string(b), "bad-pass-should-not-appear") {
				// password is in body to server; that is expected. Test output scrubbing separately.
			}
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "fresh-session", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"logs":[]}`)
		case "/getAppNames":
			n := hits.Add(1)
			if n == 1 {
				_, _ = io.WriteString(w, `{"code":401,"logs":[]}`)
				return
			}
			if !strings.Contains(r.Header.Get("Cookie"), "JSESSIONID=fresh-session") {
				t.Errorf("replay missing cookie: %q", r.Header.Get("Cookie"))
			}
			_, _ = io.WriteString(w, `["demo"]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("PLUMELOG_LOGIN_USERNAME", "alice")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "s3cret-auto-login")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "1")

	out, err := run(t, "query", "apps", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "s3cret-auto-login") {
		t.Fatalf("password leaked: %s", out)
	}
	if hits.Load() != 2 || loginHits.Load() != 1 {
		t.Fatalf("hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	if env["ok"] != true {
		t.Fatalf("%s", out)
	}
}

func TestAutoLoginFlagMissingCreds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":401,"logs":[]}`)
	}))
	defer srv.Close()

	t.Setenv("PLUMELOG_LOGIN_USERNAME", "")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")

	out, err := run(t, "query", "apps", "--base-url", srv.URL, "--auto-login")
	if err == nil {
		t.Fatal("expected error", out)
	}
	if (!strings.Contains(out, "PLUMELOG_LOGIN_USERNAME") && !strings.Contains(out, "remembered")) || strings.Contains(out, "s3cret") {
		t.Fatalf("%s", out)
	}
}

func TestAutoLoginOnlyOnce(t *testing.T) {
	var loginHits atomic.Int32
	var appHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "x", Path: "/"})
			_, _ = io.WriteString(w, `{"logs":[]}`)
		case "/getAppNames":
			appHits.Add(1)
			_, _ = io.WriteString(w, `{"code":401,"logs":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("PLUMELOG_LOGIN_USERNAME", "alice")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "s3cret-once")
	out, err := run(t, "query", "apps", "--base-url", srv.URL, "--auto-login")
	if err == nil {
		t.Fatal("expected persistent 401 after one retry", out)
	}
	if loginHits.Load() != 1 || appHits.Load() != 2 {
		t.Fatalf("login=%d apps=%d out=%s", loginHits.Load(), appHits.Load(), out)
	}
	if strings.Contains(out, "s3cret-once") {
		t.Fatalf("password leaked: %s", out)
	}
	if !strings.Contains(out, "401") {
		t.Fatalf("%s", out)
	}
}

func TestSearchContentWildcardPreservesStructured(t *testing.T) {
	for _, q := range []string{"timeout AND retry", "field:value.with.dots"} {
		out, err := run(t, "query", "search",
			"--dry-run",
			"--base-url", "http://127.0.0.1:9",
			"-q", q,
			"--content-wildcard",
			"--from-time", "1700000000000",
			"--to-time", "1700003600000",
		)
		if err != nil {
			t.Fatal(err, out)
		}
		if strings.Contains(out, "*"+q+"*") {
			t.Fatalf("force wrapped structured %q: %s", q, out)
		}
		if !strings.Contains(out, q) {
			t.Fatalf("missing original query %q: %s", q, out)
		}
	}
}

func TestSearchNoContentAutoWildcard(t *testing.T) {
	out, err := run(t, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"-q", "java.lang.OutOfMemoryError",
		"--no-content-auto-wildcard",
		"--from-time", "1700000000000",
		"--to-time", "1700003600000",
	)
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "*java.lang.OutOfMemoryError*") {
		t.Fatalf("auto wrap should be disabled: %s", out)
	}
	if !strings.Contains(out, "java.lang.OutOfMemoryError") {
		t.Fatalf("%s", out)
	}
}


func TestAutoLoginFromEncryptedCredStore(t *testing.T) {
	var hits atomic.Int32
	var loginHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			loginHits.Add(1)
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), "store-user") || !strings.Contains(string(b), "store-pass-secret") {
				t.Errorf("login body %s", b)
			}
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "from-store", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"logs":[]}`)
		case "/getAppNames":
			n := hits.Add(1)
			if n == 1 {
				_, _ = io.WriteString(w, `{"code":401,"logs":[]}`)
				return
			}
			_, _ = io.WriteString(w, `["demo"]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := credstore.Save(dir, credstore.Cred{Username: "store-user", Password: "store-pass-secret", AutoLogin: true}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PLUMELOG_LOGIN_USERNAME", "")
	t.Setenv("PLUMELOG_LOGIN_PASSWORD", "")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")

	out, err := runIn(t, dir, "query", "apps", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "store-pass-secret") {
		t.Fatalf("password leaked: %s", out)
	}
	if hits.Load() != 2 || loginHits.Load() != 1 {
		t.Fatalf("hits=%d login=%d out=%s", hits.Load(), loginHits.Load(), out)
	}
}
