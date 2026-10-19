package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAppsAppLikeFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/getAppNames" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `["zhiyi-mgy","zhiyi-iipmes","other-svc"]`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	stdout, stderr, err := runInStreams(t, dir, "query", "apps",
		"--base-url", srv.URL,
		"--app-like", "iipmes",
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	var env map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &env); jerr != nil {
		t.Fatal(jerr, stdout)
	}
	result := env["result"].(map[string]any)
	apps := result["apps"].([]any)
	if len(apps) != 1 || apps[0] != "zhiyi-iipmes" {
		t.Fatalf("apps %#v", apps)
	}
	if result["app_like"] != "iipmes" || result["count"] != float64(1) {
		t.Fatalf("meta %#v", result)
	}
}

func TestAppsEnrichOptIn(t *testing.T) {
	var clientQueries atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getAppNames":
			_, _ = io.WriteString(w, `["zhiyi-iipmes","zhiyi-mgy"]`)
		case "/clientQuery":
			clientQueries.Add(1)
			_, _ = io.WriteString(w, `{"hits":{"total":2,"hits":[{"_source":{"dtTime":1792400000000,"className":"com.iipmes.interceptor.ExceptionInterceptor","appName":"zhiyi-iipmes","content":"err"}}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	stdout, stderr, err := runInStreams(t, dir, "query", "apps",
		"--base-url", srv.URL,
		"--app-like", "zhiyi",
		"--enrich",
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if clientQueries.Load() != 2 {
		t.Fatalf("enrich queries=%d want 2", clientQueries.Load())
	}
	var env map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &env); jerr != nil {
		t.Fatal(jerr, stdout)
	}
	result := env["result"].(map[string]any)
	enrich := result["enrich"].([]any)
	if len(enrich) != 2 {
		t.Fatalf("enrich %#v", enrich)
	}
	first := enrich[0].(map[string]any)
	if first["last_log_time"] == nil || first["className_top"] == nil {
		t.Fatalf("enrich row %#v", first)
	}
}

func TestAppsSamplePreview(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getAppNames":
			_, _ = io.WriteString(w, `["zhiyi-iipmes"]`)
		case "/clientQuery":
			_, _ = io.WriteString(w, `{"hits":{"total":1,"hits":[{"_source":{"dtTime":1,"className":"com.iipmes.Foo","content":"hello","appName":"zhiyi-iipmes"}}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	stdout, _, err := runInStreams(t, dir, "query", "apps",
		"--base-url", srv.URL,
		"--sample", "zhiyi-iipmes",
	)
	if err != nil {
		t.Fatal(err, stdout)
	}
	if !strings.Contains(stdout, "com.iipmes.Foo") || !strings.Contains(stdout, `"sample"`) {
		t.Fatalf("sample missing: %s", stdout)
	}
}

func TestFilterAppLikeUnit(t *testing.T) {
	got := filterAppLike([]string{"A-mgy", "B-iipmes", "C"}, "IIPMES")
	if len(got) != 1 || got[0] != "B-iipmes" {
		t.Fatalf("%v", got)
	}
}
