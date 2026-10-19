package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTraceGetEmptyHintsRunLogSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "trace=true") {
			t.Errorf("expected trace=true query, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"took":1,"hits":{"total":0,"hits":[]}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	stdout, stderr, err := runInStreams(t, dir, "trace", "get",
		"--base-url", srv.URL,
		"--trace-id", "2456e78e193205535aecef6ca699bba4",
		"--size", "5",
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	var env map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &env); jerr != nil {
		t.Fatal(jerr, stdout)
	}
	if env["ok"] != true {
		t.Fatalf("still ok on empty hits: %s", stdout)
	}
	hint, _ := env["hint"].(string)
	if !strings.Contains(hint, "query search") || !strings.Contains(hint, "--trace-id") {
		t.Fatalf("want run-log search hint: %q (stderr=%q)", hint, stderr)
	}
	if !strings.Contains(hint, "2456e78e193205535aecef6ca699bba4") {
		t.Fatalf("hint should include trace id: %q", hint)
	}
}

func TestTraceGetWithHitsNoHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"hits":{"total":1,"hits":[{"_id":"1"}]}}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	stdout, _, err := runInStreams(t, dir, "trace", "get",
		"--base-url", srv.URL,
		"--trace-id", "tid-ok",
	)
	if err != nil {
		t.Fatal(err, stdout)
	}
	var env map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &env); jerr != nil {
		t.Fatal(jerr, stdout)
	}
	if hint, _ := env["hint"].(string); strings.Contains(hint, "query search") {
		t.Fatalf("unexpected hint when hits>0: %q", hint)
	}
}
