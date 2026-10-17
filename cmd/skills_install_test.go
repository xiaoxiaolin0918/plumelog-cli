package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverInstallableSkills(t *testing.T) {
	root := t.TempDir()
	mustMkSkill := func(name string, withMD bool) {
		d := filepath.Join(root, name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if withMD {
			if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mustMkSkill("plumelog-shared", true)
	mustMkSkill("plumelog-query", true)
	mustMkSkill("plumelog-empty", false)
	mustMkSkill("other-skill", true)
	if err := os.WriteFile(filepath.Join(root, "plumelog-notadir"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	names, err := discoverInstallableSkills(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "plumelog-query" || names[1] != "plumelog-shared" {
		t.Fatalf("got %v", names)
	}

	names, err = discoverInstallableSkills(root, []string{"plumelog-shared"})
	if err != nil || len(names) != 1 || names[0] != "plumelog-shared" {
		t.Fatalf("filter: %v %v", names, err)
	}

	if _, err := discoverInstallableSkills(root, []string{"plumelog-missing"}); err == nil {
		t.Fatal("expected missing skill error")
	}
}

func TestDefaultSkillsInstallDir(t *testing.T) {
	d := defaultSkillsInstallDir()
	if filepath.Base(d) != "skills" || filepath.Base(filepath.Dir(d)) != ".agents" {
		t.Fatalf("unexpected default dir %q", d)
	}
}

func TestInstallOneSkillCopyAndSkip(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	src := filepath.Join(srcRoot, "plumelog-shared")
	if err := os.MkdirAll(filepath.Join(src, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "references", "a.md"), []byte("ref\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dstRoot, "plumelog-shared")

	ins, sk, err := installOneSkill(src, dst, false, false, false)
	if err != nil || sk != nil || ins == nil || ins.Mode != "copy" {
		t.Fatalf("install: ins=%v sk=%v err=%v", ins, sk, err)
	}
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "references", "a.md")); err != nil {
		t.Fatal(err)
	}

	ins, sk, err = installOneSkill(src, dst, false, false, false)
	if err != nil || ins != nil || sk == nil || sk.Reason != "exists" {
		t.Fatalf("skip: ins=%v sk=%v err=%v", ins, sk, err)
	}

	ins, sk, err = installOneSkill(src, dst, false, true, true)
	if err != nil || sk != nil || ins == nil {
		t.Fatalf("dry-run force: ins=%v sk=%v err=%v", ins, sk, err)
	}
}

func TestInstallSkillsReportsExistingSkips(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	for _, n := range []string{"plumelog-alpha", "plumelog-beta"} {
		d := filepath.Join(srcRoot, n)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("# "+n+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var warn bytes.Buffer
	result, err := installSkills(srcRoot, dstRoot, nil, false, false, false, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if result["installed_count"].(int) != 2 || result["skipped_count"].(int) != 0 {
		t.Fatalf("first: %+v", result)
	}
	if warn.Len() != 0 {
		t.Fatalf("first warn: %s", warn.String())
	}

	warn.Reset()
	result, err = installSkills(srcRoot, dstRoot, nil, false, false, false, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if result["installed_count"].(int) != 0 || result["skipped_count"].(int) != 2 {
		t.Fatalf("rerun: %+v", result)
	}
	if !strings.Contains(warn.String(), "--force") {
		t.Fatalf("warn missing --force: %s", warn.String())
	}
	hint, _ := result["hint"].(string)
	if !strings.Contains(hint, "--force") {
		t.Fatalf("hint: %q", hint)
	}
}

func TestSkillsCLIListReadPath(t *testing.T) {
	root := NewRoot()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	jar := filepath.Join(dir, "cookies.txt")

	run := func(args ...string) (string, error) {
		r := NewRoot()
		var stdout bytes.Buffer
		r.SetOut(&stdout)
		r.SetErr(ioDiscard{})
		full := append([]string{"--config", cfg, "--cookie-jar", jar}, args...)
		r.SetArgs(full)
		err := r.Execute()
		return stdout.String(), err
	}
	_ = root

	out, err := run("skills", "list")
	if err != nil {
		t.Fatalf("list: %v / %s", err, out)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("json: %v / %s", err, out)
	}
	if env["ok"] != true {
		t.Fatalf("list not ok: %s", out)
	}
	result, _ := env["result"].(map[string]any)
	skills, _ := result["skills"].([]any)
	if len(skills) < 1 {
		t.Fatalf("expected skills from repo: %s", out)
	}

	out, err = run("skills", "path")
	if err != nil {
		t.Fatalf("path: %v / %s", err, out)
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	result, _ = env["result"].(map[string]any)
	path, _ := result["path"].(string)
	if path == "" || !strings.Contains(path, "skills") {
		t.Fatalf("path: %s", out)
	}

	name, _ := skills[0].(string)
	out, err = run("skills", "read", name)
	if err != nil {
		t.Fatalf("read: %v / %s", err, out)
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	result, _ = env["result"].(map[string]any)
	content, _ := result["content"].(string)
	if !strings.Contains(content, "plumelog") && !strings.Contains(content, "name:") {
		t.Fatalf("content unexpected: %q", content[:min(80, len(content))])
	}

	out, err = run("skills", "path", name)
	if err != nil {
		t.Fatalf("path name: %v / %s", err, out)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestSkillsUpdateForceRefreshes(t *testing.T) {
	srcRoot := t.TempDir()
	dstRoot := t.TempDir()
	skill := filepath.Join(srcRoot, "plumelog-shared")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: plumelog-shared\nversion: 0.1.0\n---\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// seed outdated install
	inst := filepath.Join(dstRoot, "plumelog-shared")
	if err := os.MkdirAll(inst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inst, "SKILL.md"), []byte("---\nname: plumelog-shared\nversion: 0.0.1\n---\nstale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// bump source
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: plumelog-shared\nversion: 0.2.0\n---\nfresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	result, err := installSkills(srcRoot, dstRoot, nil, false, true, false, &warn)
	if err != nil {
		t.Fatal(err)
	}
	if result["installed_count"].(int) != 1 {
		t.Fatalf("%+v", result)
	}
	got, err := os.ReadFile(filepath.Join(inst, "SKILL.md"))
	if err != nil || !strings.Contains(string(got), "0.2.0") || !strings.Contains(string(got), "fresh") {
		t.Fatalf("not refreshed: %s err=%v", got, err)
	}
}

func TestReadSkillVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte("---\nname: x\nversion: 1.2.3\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := readSkillVersion(p); v != "1.2.3" {
		t.Fatalf("got %q", v)
	}
}