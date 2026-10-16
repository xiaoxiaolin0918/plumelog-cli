package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestJoinAndRedactURL(t *testing.T) {
	q := url.Values{}
	q.Set("size", "10")
	q.Set("adminPassWord", "hunter2")
	raw, err := JoinURL("http://localhost:8891/", "/plumelog", "clientQuery", RedactQuery(q))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "hunter2") {
		t.Fatalf("password leaked: %s", raw)
	}
	if !strings.Contains(raw, "http://localhost:8891/plumelog/clientQuery?") {
		t.Fatalf("url %s", raw)
	}
	if !strings.Contains(raw, "adminPassWord=") {
		t.Fatalf("url %s", raw)
	}
	if !PathAlreadyPrefixed("/plumelog/login") {
		t.Fatal("expected prefixed path")
	}
}

func TestPreviewRedactsPasswordAndCookie(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:8891", Prefixes: []string{"", "/plumelog"}}
	env, err := c.Preview(Request{
		Method: "POST",
		Path:   "/login",
		Body:   []byte(`{"username":"alice","password":"s3cret"}`),
	}, []string{"JSESSIONID"}, []string{"--yes"}, "write")
	if err != nil {
		t.Fatal(err)
	}
	if !env.DryRun || env.Method != "POST" {
		t.Fatalf("%+v", env)
	}
	if env.Headers["Cookie"] != "<redacted>" {
		t.Fatalf("cookie header %#v", env.Headers)
	}
	if strings.Contains(string(env.Body), "s3cret") {
		t.Fatalf("body %s", env.Body)
	}
	if len(env.FallbackURLs) != 1 || !strings.Contains(env.FallbackURLs[0], "/plumelog/login") {
		t.Fatalf("fallback %#v", env.FallbackURLs)
	}
}

func TestFallbackToPlumelogPrefix(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/plumelog/clientQuery" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"hits":{}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := &Client{
		BaseURL:  srv.URL,
		Prefixes: []string{"", "/plumelog"},
		HTTP:     srv.Client(),
	}
	resp, err := c.Do(context.Background(), Request{Method: "POST", Path: "/clientQuery", Body: []byte(`{"query":{"match_all":{}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 || !strings.Contains(resp.FinalURL, "/plumelog/clientQuery") {
		t.Fatalf("status %d url %s", resp.Status, resp.FinalURL)
	}
	if len(paths) != 2 || paths[0] != "/clientQuery" || paths[1] != "/plumelog/clientQuery" {
		t.Fatalf("paths %v", paths)
	}
}

func TestCookieJarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.txt")
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

	jar, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{BaseURL: srv.URL, Prefixes: []string{""}, HTTP: &http.Client{Jar: jar}, Jar: jar}
	if _, err := c.Do(context.Background(), Request{Method: "POST", Path: "/login", Body: []byte(`{"username":"a","password":"b"}`)}); err != nil {
		t.Fatal(err)
	}
	jar2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	c2 := &Client{BaseURL: srv.URL, Prefixes: []string{""}, HTTP: &http.Client{Jar: jar2}, Jar: jar2}
	resp, err := c2.Do(context.Background(), Request{Method: "POST", Path: "/getRunModel"})
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != `"redis"` {
		t.Fatalf("body %s", resp.Body)
	}
	if !strings.Contains(seen, "JSESSIONID=secret-cookie-value") {
		t.Fatalf("cookie header %q", seen)
	}
	names := jar2.Names(mustURL(t, srv.URL+"/"))
	if len(names) != 1 || names[0] != "JSESSIONID" {
		t.Fatalf("names %v", names)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("mode %o", fi.Mode().Perm())
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# Netscape HTTP Cookie File") {
		t.Fatalf("jar header missing: %s", raw)
	}
}

func TestResultCode(t *testing.T) {
	code, ok := ResultCode([]byte(`{"code":401,"logs":[]}`))
	if !ok || code != 401 {
		t.Fatalf("%d %v", code, ok)
	}
	if _, ok := ResultCode([]byte(`"redis"`)); ok {
		t.Fatal("string should not have a code")
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
