package cmd

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSearchIndexAutoAlignsDayWindow(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, err := runInStreams(t, dir, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--index", "plume_log_run_20261016",
		"--size", "5",
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if !strings.Contains(stderr, "note: --index plume_log_run_20261016") || !strings.Contains(stderr, "2026-10-16") {
		t.Fatalf("stderr note missing: %q", stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err, stdout)
	}
	rawURL := env["url"].(string)
	if !strings.Contains(rawURL, "/query?") || !strings.Contains(rawURL, "index=plume_log_run_20261016") {
		t.Fatalf("url %s", rawURL)
	}
	body := env["body"].(map[string]any)
	must := body["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	var gte, lt float64
	found := false
	for _, clause := range must {
		m := clause.(map[string]any)
		rng, ok := m["range"].(map[string]any)
		if !ok {
			continue
		}
		dt, ok := rng["dtTime"].(map[string]any)
		if !ok {
			continue
		}
		gte = dt["gte"].(float64)
		lt = dt["lt"].(float64)
		found = true
	}
	if !found {
		t.Fatalf("no dtTime range in body: %#v", must)
	}
	loc := time.Local
	wantFrom := time.Date(2026, 10, 16, 0, 0, 0, 0, loc).UnixMilli()
	wantTo := time.Date(2026, 10, 17, 0, 0, 0, 0, loc).UnixMilli()
	if int64(gte) != wantFrom || int64(lt) != wantTo {
		t.Fatalf("window gte=%v lt=%v want %d-%d (ms strings %s %s)", gte, lt, wantFrom, wantTo,
			strconv.FormatInt(wantFrom, 10), strconv.FormatInt(wantTo, 10))
	}
}

func TestSearchIndexWithRangeWarnsHistorical(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, err := runInStreams(t, dir, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--index", "plume_log_run_20260101",
		"--range", "day",
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if !strings.Contains(stderr, "warning:") || !strings.Contains(stderr, "--range") {
		t.Fatalf("expected historical+range warning: %q", stderr)
	}
}

func TestSearchIndexExplicitTimeSkipsAutoAlign(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, err := runInStreams(t, dir, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--index", "plume_log_run_20261016",
		"--from-time", "2026-10-16 10:00:00",
		"--to-time", "2026-10-16 11:00:00",
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if strings.Contains(stderr, "using index day") {
		t.Fatalf("should not auto-align when explicit times given: %q", stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	body := env["body"].(map[string]any)
	must := body["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	blob, _ := json.Marshal(must)
	if !strings.Contains(string(blob), "dtTime") {
		t.Fatalf("body %s", blob)
	}
}