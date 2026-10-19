package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmptyBodyIsFailureWithResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clientQuery" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		// empty body — missing index window
	}))
	defer srv.Close()

	dir := t.TempDir()
	stdout, stderr, err := runInStreams(t, dir, "query", "search",
		"--base-url", srv.URL,
		"--size", "1",
	)
	if err == nil {
		t.Fatalf("expected non-nil error, stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout should be empty on failure: %q", stdout)
	}
	var env map[string]any
	if jerr := json.Unmarshal([]byte(stderr), &env); jerr != nil {
		t.Fatalf("stderr JSON: %v (%q)", jerr, stderr)
	}
	if env["ok"] != false {
		t.Fatalf("want ok:false: %s", stderr)
	}
	errObj, _ := env["error"].(map[string]any)
	if errObj == nil || errObj["type"] != "server" {
		t.Fatalf("want server error: %#v", errObj)
	}
	if errObj["subtype"] != "empty_body" {
		t.Fatalf("subtype %#v", errObj["subtype"])
	}
	if !strings.Contains(fmtString(errObj["message"]), "empty body") && !strings.Contains(fmtString(errObj["message"]), "no matching index") {
		t.Fatalf("message: %#v", errObj["message"])
	}
	result, ok := env["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %s", stderr)
	}
	hits, ok := result["hits"].(map[string]any)
	if !ok {
		t.Fatalf("missing result.hits: %#v", result)
	}
	if hits["total"] != float64(0) {
		t.Fatalf("total %#v", hits["total"])
	}
}

func fmtString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}