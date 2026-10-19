package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSearchRangeExpandsBodyWindow(t *testing.T) {
	dir := t.TempDir()
	before := time.Now()
	stdout, stderr, err := runInStreams(t, dir, "query", "search",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--range", "week",
		"-q", `"Column index out of range"`,
		"--size", "5",
	)
	after := time.Now()
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err, stdout)
	}
	rawURL := env["url"].(string)
	if !strings.Contains(rawURL, "/query?") || !strings.Contains(rawURL, "range=week") {
		t.Fatalf("url %s", rawURL)
	}
	body := env["body"].(map[string]any)
	must := body["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	var gte, lt float64
	var qs map[string]any
	for _, clause := range must {
		m := clause.(map[string]any)
		if rng, ok := m["range"].(map[string]any); ok {
			if dt, ok := rng["dtTime"].(map[string]any); ok {
				gte = dt["gte"].(float64)
				lt = dt["lt"].(float64)
			}
		}
		if q, ok := m["query_string"].(map[string]any); ok {
			qs = q
		}
	}
	if qs == nil || qs["query"] != `"Column index out of range"` {
		t.Fatalf("query_string not preserved: %#v", qs)
	}
	from := time.UnixMilli(int64(gte))
	to := time.UnixMilli(int64(lt))
	span := to.Sub(from)
	if span < 6*24*time.Hour || span > 8*24*time.Hour {
		t.Fatalf("body dtTime span %v want ~7d (gte=%v lt=%v); stderr=%q", span, gte, lt, stderr)
	}
	if to.Before(before.Add(-time.Minute)) || to.After(after.Add(time.Minute)) {
		t.Fatalf("to-time not near now: %v", to)
	}
}
