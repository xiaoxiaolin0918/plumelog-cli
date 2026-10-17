package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/client"
)

func TestWebLoginDryRunNoSecrets(t *testing.T) {
	out, err := run(t, "auth", "login", "--web", "--dry-run", "--base-url", "http://127.0.0.1:9")
	if err != nil {
		t.Fatal(err, out)
	}
	if strings.Contains(out, "s3cret") {
		t.Fatalf("leaked: %s", out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	if env["ok"] != true || env["dry_run"] != true {
		t.Fatalf("%s", out)
	}
	result, _ := env["result"].(map[string]any)
	if result["web_login"] != true || result["bind"] != "127.0.0.1" {
		t.Fatalf("%s", out)
	}
}

func TestWebLoginBrowserAliasDryRun(t *testing.T) {
	out, err := run(t, "auth", "login", "--browser", "--dry-run", "--base-url", "http://127.0.0.1:9")
	if err != nil {
		t.Fatal(err, out)
	}
	if !strings.Contains(out, `"web_login":true`) {
		t.Fatalf("%s", out)
	}
}

func TestWebLoginBindLoopbackTokenAndPersist(t *testing.T) {
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		gotBody.Store(string(body))
		http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "web-cookie-value", Path: "/", HttpOnly: true})
		_, _ = io.WriteString(w, `{"logs":[]}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	a := newTestApp(t, dir, srv.URL)
	cmd := a.newLoginCmd()
	cmd.SetContext(context.Background())
	var outBuf strings.Builder
	cmd.SetOut(&outBuf)
	cmd.SetErr(io.Discard)

	ready := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		u := <-ready
		host, _, err := net.SplitHostPort(strings.TrimPrefix(strings.Split(u, "/cli-login/")[0], "http://"))
		if err != nil {
			errCh <- err
			return
		}
		if host != "127.0.0.1" {
			errCh <- errors.New("not loopback: " + host)
			return
		}
		// Wrong token path must 404.
		badURL := "http://" + strings.Split(strings.TrimPrefix(u, "http://"), "/")[0] + "/cli-login/00000000000000000000000000000000/"
		resp, err := http.Get(badURL)
		if err != nil {
			errCh <- err
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			errCh <- errors.New("want 404 for bad token")
			return
		}
		form := url.Values{"username": {"webuser"}, "password": {"web-pass-secret"}}
		resp, err = http.PostForm(u, form)
		if err != nil {
			errCh <- err
			return
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 {
			errCh <- errors.New("post status")
			return
		}
		if strings.Contains(string(b), "web-pass-secret") {
			errCh <- errors.New("html leaked password")
			return
		}
		if !strings.Contains(string(b), "登录成功") {
			errCh <- errors.New("missing success page")
			return
		}
		errCh <- nil
	}()

	err := a.runWebLogin(cmd, false, webLoginDeps{
		Timeout: 15 * time.Second,
		OpenBrowser: func(u string) error {
			ready <- u
			return nil
		},
	})
	if err != nil {
		t.Fatal(err, outBuf.String())
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	out := outBuf.String()
	if strings.Contains(out, "web-pass-secret") || strings.Contains(out, "web-cookie-value") {
		t.Fatalf("leaked: %s", out)
	}
	if !strings.Contains(out, `"via":"web"`) || !strings.Contains(out, `"has_cookie":true`) {
		t.Fatalf("%s", out)
	}
	body, _ := gotBody.Load().(string)
	if !strings.Contains(body, "webuser") || !strings.Contains(body, "web-pass-secret") {
		t.Fatalf("upstream body %q", body)
	}
	jarBytes, err := os.ReadFile(filepath.Join(dir, "cookies.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(jarBytes), "web-pass-secret") {
		t.Fatal("password landed in jar file")
	}
	if !strings.Contains(string(jarBytes), "JSESSIONID") {
		t.Fatalf("jar missing session: %s", jarBytes)
	}
}

func TestWebLoginRequiresForceWhenJarPresent(t *testing.T) {
	dir := t.TempDir()
	cookiePath := filepath.Join(dir, "cookies.txt")
	content := "# Netscape HTTP Cookie File\n127.0.0.1\tFALSE\t/\tFALSE\t0\tJSESSIONID\told\n"
	if err := os.WriteFile(cookiePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newTestApp(t, dir, "http://127.0.0.1:9")
	cmd := a.newLoginCmd()
	cmd.SetContext(context.Background())
	var outBuf, errBuf strings.Builder
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	err := a.runWebLogin(cmd, false, webLoginDeps{
		Timeout:     time.Second,
		OpenBrowser: func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("expected error when jar present without --force")
	}
	msg := err.Error() + outBuf.String() + errBuf.String()
	if !strings.Contains(msg, "force") {
		t.Fatalf("want --force hint: %s", msg)
	}
}

func TestWebLoginTimeoutClosesListener(t *testing.T) {
	dir := t.TempDir()
	a := newTestApp(t, dir, "http://127.0.0.1:9")
	cmd := a.newLoginCmd()
	cmd.SetContext(context.Background())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	start := time.Now()
	err := a.runWebLogin(cmd, false, webLoginDeps{
		Timeout:     250 * time.Millisecond,
		OpenBrowser: func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("%v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout too slow: %s", time.Since(start))
	}
}

func TestWebLoginListenIsLoopbackOnly(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	host, _, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" {
		t.Fatalf("got %s want 127.0.0.1 (never 0.0.0.0)", host)
	}
}

func TestWebLoginFormHTMLChinese(t *testing.T) {
	html := webLoginFormHTML()
	for _, want := range []string{"lang=\"zh-CN\"", "本机登录", "用户名", "密码", "登录", "127.0.0.1"} {
		if !strings.Contains(html, want) {
			t.Fatalf("form HTML missing %q", want)
		}
	}
	if strings.Contains(html, "cdn.") || strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Fatal("form HTML must not load external resources")
	}
	if strings.Contains(html, "Sign in") || strings.Contains(html, "Username") {
		t.Fatal("form HTML still has English chrome")
	}
}

func TestWebLoginOKHTMLShowsBaseURL(t *testing.T) {
	html := webLoginOKHTML("http://example.test/pl", `C:\cookies.txt`)
	for _, want := range []string{"登录成功", "http://example.test/pl", `C:\cookies.txt`, "base_url"} {
		if !strings.Contains(html, want) {
			t.Fatalf("ok HTML missing %q", want)
		}
	}
}

func TestWebLoginFailHTMLChinese(t *testing.T) {
	html := webLoginFailHTML(errors.New("invalid login (402)"))
	if !strings.Contains(html, "登录失败") {
		t.Fatalf("missing fail title: %s", html)
	}
	if !strings.Contains(html, "用户名或密码错误") {
		t.Fatalf("want zh invalid-login: %s", html)
	}
	if strings.Contains(html, "<script") {
		t.Fatal("unexpected script")
	}
}

func newTestApp(t *testing.T, dir, baseURL string) *App {
	t.Helper()
	cfgPath := filepath.Join(dir, "config.json")
	cookiePath := filepath.Join(dir, "cookies.txt")
	jar, err := client.Open(cookiePath)
	if err != nil {
		t.Fatal(err)
	}
	hc := clientHTTP(10*time.Second, jar)
	a := &App{
		configPath: cfgPath,
		cookiePath: cookiePath,
		baseURL:    strings.TrimRight(baseURL, "/"),
		prefixMode: "",
		format:     "json",
		timeout:    10 * time.Second,
		jar:        jar,
		client: &client.Client{
			BaseURL:  strings.TrimRight(baseURL, "/"),
			Prefixes: []string{""},
			HTTP:     &hc,
			Jar:      jar,
		},
	}
	a.cfg.BaseURL = a.baseURL
	return a
}
