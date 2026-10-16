package cmd

import (
	"bytes"
	"errors"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"plumelog-cli/internal/risk"
)

func TestCommandTreeRisk(t *testing.T) {
	root := NewRoot()
	required := map[string]string{
		"auth login":            "write",
		"auth logout":           "write",
		"auth status":           "read",
		"doctor":                "read",
		"config get":            "read",
		"config set":            "write",
		"config path":           "read",
		"config show":           "read",
		"query search":          "read",
		"query dsl":             "read",
		"query apps":            "read",
		"trace get":             "read",
		"app list":              "read",
		"app extend list":       "read",
		"app extend add":        "write",
		"app extend del":        "high-risk-write",
		"alarm list":            "read",
		"alarm save":            "write",
		"alarm delete":          "high-risk-write",
		"alarm messages":        "read",
		"admin queue counts":    "read",
		"admin queue clear":     "high-risk-write",
		"admin index info":      "read",
		"admin index delete":    "high-risk-write",
		"admin index reset":     "high-risk-write",
		"admin run-model":       "read",
		"admin delete-by-query": "high-risk-write",
		"api":                   "varies",
		"version":               "read",
	}
	for path, want := range required {
		cmd, _, err := root.Find(strings.Split(path, " "))
		if err != nil || cmd == nil {
			t.Fatalf("missing %s: %v", path, err)
		}
		if cmd.Annotations["risk"] != want {
			t.Fatalf("%s risk %q", path, cmd.Annotations["risk"])
		}
		if !strings.Contains(cmd.Short, "Risk:") {
			t.Fatalf("%s short %q", path, cmd.Short)
		}
	}
	help := new(bytes.Buffer)
	dir := t.TempDir()
	root.SetOut(help)
	root.SetArgs([]string{
		"--config", filepath.Join(dir, "config.json"),
		"--cookie-jar", filepath.Join(dir, "cookies.txt"),
		"--help",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	text := help.String()
	for _, needle := range []string{"Quick start", "high-risk-write", "JSESSIONID", "--dry-run"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("root help missing %q", needle)
		}
	}
}

func TestSearchDryRunShapesRequest(t *testing.T) {
	out, err := run(t, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--app", "demo",
		"--level", "ERROR",
		"-q", "timeout",
		"--from-time", "1700000000000",
		"--to-time", "1700003600000",
		"--size", "10",
		"--from", "0",
	)
	if err != nil {
		t.Fatal(err, out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	if env["ok"] != true || env["dry_run"] != true || env["method"] != "POST" || env["risk"] != "read" {
		t.Fatalf("%s", out)
	}
	rawURL := env["url"].(string)
	if !strings.Contains(rawURL, "http://127.0.0.1:9/clientQuery?") {
		t.Fatalf("url %s", rawURL)
	}
	for _, q := range []string{"clientStartDate=1700000000000", "clientEndDate=1700003600000", "size=10", "from=0"} {
		if !strings.Contains(rawURL, q) {
			t.Fatalf("url missing %s in %s", q, rawURL)
		}
	}
	body := env["body"].(map[string]any)
	must := body["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	blob, _ := json.Marshal(must)
	if !strings.Contains(string(blob), "demo") || !strings.Contains(string(blob), "ERROR") || !strings.Contains(string(blob), "timeout") {
		t.Fatalf("must %s", blob)
	}
	sort := body["sort"].([]any)
	if sort[0].(map[string]any)["dtTime"] != "desc" {
		t.Fatalf("sort %#v", sort)
	}
	fb := env["fallback_urls"].([]any)
	if !strings.Contains(fb[0].(string), "/plumelog/clientQuery?") {
		t.Fatalf("fallback %#v", fb)
	}
}

func TestTraceDryRunUsesTraceFlagAndAscSort(t *testing.T) {
	out, err := run(t, "trace", "get", "--trace-id", "tid-9", "--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--from-time", "1700000000000", "--to-time", "1700003600000")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, "trace=true") || !strings.Contains(out, "tid-9") {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(out, `"dtTime":"asc"`) || !strings.Contains(out, `"seq":"asc"`) {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(out, "size=1000") {
		t.Fatalf("%s", out)
	}
}

func TestHighRiskDryRunDoesNotSendOrLeakPassword(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	out, err := run(t, "admin", "queue", "clear", "--dry-run",
		"--base-url", "http://"+addr,
		"--admin-password", "hunter2secret")
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "hunter2secret") {
		t.Fatalf("leaked password: %s", out)
	}
	if !strings.Contains(out, "adminPassWord=") || !strings.Contains(out, "/deleteQueue") {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(out, "--yes") {
		t.Fatalf("expected would_require --yes: %s", out)
	}

	refused, err := run(t, "admin", "queue", "clear",
		"--base-url", "http://"+addr,
		"--admin-password", "hunter2secret")
	if err == nil {
		t.Fatal("expected refusal")
	}
	var pe *PrintedError
	if !errors.As(err, &pe) || pe.ExitCode() != risk.ExitConfirmationRequired {
		t.Fatalf("want exit 10 PrintedError, got %v", err)
	}
	if strings.Contains(refused, "hunter2secret") || !strings.Contains(refused, "confirmation_required") {
		t.Fatalf("%s", refused)
	}
	if !strings.Contains(refused, "\"type\":\"confirmation\"") || !strings.Contains(refused, "admin queue clear") {
		t.Fatalf("%s", refused)
	}
	if strings.Contains(refused, "connection refused") || strings.Contains(refused, "request failed") {
		t.Fatalf("gate should not send: %s", refused)
	}
}

func TestConfirmationGateExit10WithoutAdmin(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	out, err := run(t, "admin", "index", "reset",
		"--base-url", "http://"+addr,
		"--index", "plume_log_run_20240101")
	if err == nil {
		t.Fatal("expected refusal")
	}
	var pe *PrintedError
	if !errors.As(err, &pe) || pe.ExitCode() != risk.ExitConfirmationRequired {
		t.Fatalf("want exit 10, got %v", err)
	}
	if !strings.Contains(out, "confirmation_required") || !strings.Contains(out, "add --yes to confirm") {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(out, "admin index reset") {
		t.Fatalf("missing action: %s", out)
	}
}

func TestLoginDryRunAndCookieRoundTrip(t *testing.T) {
	out, err := run(t, "auth", "login", "--username", "alice", "--password", "s3cret",
		"--dry-run", "--base-url", "http://127.0.0.1:9")
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "s3cret") || !strings.Contains(out, "***") {
		t.Fatalf("%s", out)
	}

	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "secret-cookie-value", Path: "/", HttpOnly: true})
			_, _ = io.WriteString(w, `{"logs":[]}`)
		case "/getRunModel":
			seen = r.Header.Get("Cookie")
			_, _ = io.WriteString(w, `"redis"`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	loginOut, err := runIn(t, dir, "auth", "login", "--username", "alice", "--password", "s3cret", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, loginOut)
	}
	if strings.Contains(loginOut, "s3cret") || strings.Contains(loginOut, "secret-cookie-value") {
		t.Fatalf("login output leaked: %s", loginOut)
	}
	if !strings.Contains(loginOut, `"auth":"session"`) || !strings.Contains(loginOut, `"has_cookie":true`) {
		t.Fatalf("%s", loginOut)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cfg), "s3cret") {
		t.Fatalf("config stored password: %s", cfg)
	}

	statusOut, err := runIn(t, dir, "auth", "status", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, statusOut)
	}
	if strings.Contains(statusOut, "secret-cookie-value") || !strings.Contains(statusOut, `"has_cookie":true`) {
		t.Fatalf("%s", statusOut)
	}

	probeOut, err := runIn(t, dir, "api", "POST", "/getRunModel", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, probeOut)
	}
	if !strings.Contains(seen, "JSESSIONID=secret-cookie-value") {
		t.Fatalf("server saw %q", seen)
	}
	if strings.Contains(probeOut, "secret-cookie-value") {
		t.Fatalf("probe leaked cookie: %s", probeOut)
	}
}

func TestConfigSetRefusesSecrets(t *testing.T) {
	out, err := run(t, "config", "set", "login_password", "s3cret")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(out, "s3cret") || !strings.Contains(out, "refusing") {
		t.Fatalf("%s", out)
	}
	dir := t.TempDir()
	ok, err := runIn(t, dir, "config", "set", "base_url", "http://127.0.0.1:8891")
	if err != nil {
		t.Fatal(err, ok)
	}
	got, err := runIn(t, dir, "config", "get", "base_url")
	if err != nil {
		t.Fatal(err, got)
	}
	if !strings.Contains(got, "http://127.0.0.1:8891") {
		t.Fatalf("%s", got)
	}
	filtered, err := runIn(t, dir, "--jq", ".result.value", "config", "get", "base_url")
	if err != nil {
		t.Fatal(err, filtered)
	}
	if strings.TrimSpace(filtered) != `"http://127.0.0.1:8891"` {
		t.Fatalf("jq %s", filtered)
	}
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runIn(t, t.TempDir(), args...)
}

func runIn(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	root := NewRoot()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	full := append([]string{
		"--config", filepath.Join(dir, "config.json"),
		"--cookie-jar", filepath.Join(dir, "cookies.txt"),
		"--format", "json",
	}, args...)
	root.SetArgs(full)
	err := root.Execute()
	return stdout.String(), err
}
