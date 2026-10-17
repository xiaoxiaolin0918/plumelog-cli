package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSkillsListCheckSemverDirection(t *testing.T) {
	bundledRoot := findDiskSkills()
	if bundledRoot == "" {
		t.Skip("no on-disk skills")
	}
	// pick first plumelog-* skill
	entries, err := os.ReadDir(bundledRoot)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "plumelog-") {
			name = e.Name()
			break
		}
	}
	if name == "" {
		t.Fatal("no skill")
	}
	dir := t.TempDir()
	// Exercise list --check with HOME override.
	home := t.TempDir()
	agents := filepath.Join(home, ".agents", "skills", name)
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, "SKILL.md"), []byte("---\nname: "+name+"\nversion: 0.0.1\n---\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	stdout, stderr, err := runInStreams(t, dir, "skills", "list", "--check")
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err, stdout)
	}
	result := env["result"].(map[string]any)
	checks, _ := result["check"].([]any)
	found := false
	for _, c := range checks {
		m := c.(map[string]any)
		if m["name"] == name {
			found = true
			if m["status"] != "outdated" {
				t.Fatalf("want outdated got %#v (bundled=%v installed=%v)", m, m["bundled_version"], m["installed_version"])
			}
		}
	}
	if !found {
		t.Fatalf("skill %s not in check: %s", name, stdout)
	}

	// ahead
	if err := os.WriteFile(filepath.Join(agents, "SKILL.md"), []byte("---\nname: "+name+"\nversion: 99.0.0\n---\nahead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err = runInStreams(t, dir, "skills", "list", "--check")
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	result = env["result"].(map[string]any)
	checks, _ = result["check"].([]any)
	for _, c := range checks {
		m := c.(map[string]any)
		if m["name"] == name && m["status"] != "ahead" {
			t.Fatalf("want ahead got %#v", m)
		}
	}
}

func TestDSLIndexAutoAlignsBodyDtTime(t *testing.T) {
	dir := t.TempDir()
	dslPath := filepath.Join(dir, "q.json")
	if err := os.WriteFile(dslPath, []byte(`{"query":{"match_all":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runInStreams(t, dir, "query", "dsl",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--index", "plume_log_run_20261016",
		"--file", dslPath,
		"--size", "5",
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if !strings.Contains(stderr, "note: --index plume_log_run_20261016") {
		t.Fatalf("stderr note missing: %q", stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err, stdout)
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
		t.Fatalf("no dtTime in dsl body: %#v", body)
	}
	loc := time.Local
	wantFrom := time.Date(2026, 10, 16, 0, 0, 0, 0, loc).UnixMilli()
	wantTo := time.Date(2026, 10, 17, 0, 0, 0, 0, loc).UnixMilli()
	if int64(gte) != wantFrom || int64(lt) != wantTo {
		t.Fatalf("window gte=%v lt=%v want %d-%d (%s %s)", gte, lt, wantFrom, wantTo,
			strconv.FormatInt(wantFrom, 10), strconv.FormatInt(wantTo, 10))
	}
}

func TestDSLIndexExplicitTimeSkipsBodyRewrite(t *testing.T) {
	dir := t.TempDir()
	dslPath := filepath.Join(dir, "q.json")
	if err := os.WriteFile(dslPath, []byte(`{"query":{"match_all":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runInStreams(t, dir, "query", "dsl",
		"--dry-run",
		"--base-url", "http://127.0.0.1:9",
		"--index", "plume_log_run_20261016",
		"--from-time", "2026-10-16 10:00:00",
		"--to-time", "2026-10-16 11:00:00",
		"--file", dslPath,
	)
	if err != nil {
		t.Fatal(err, stdout, stderr)
	}
	if strings.Contains(stderr, "using index day") {
		t.Fatalf("should not auto-align: %q", stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatal(err)
	}
	body := env["body"].(map[string]any)
	q := body["query"].(map[string]any)
	if _, ok := q["match_all"]; !ok {
		t.Fatalf("raw body should stay match_all when explicit time: %#v", body)
	}
}
