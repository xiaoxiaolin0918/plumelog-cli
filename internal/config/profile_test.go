package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateProfileName(t *testing.T) {
	for _, name := range []string{"default", "dev", "prod", "a", "Dev_1", "x-y"} {
		if err := ValidateProfileName(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, name := range []string{"", "../x", "a/b", `a\b`, ".", "..", "-bad", "has space", "weird!"} {
		if err := ValidateProfileName(name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
}

func TestPathsForProfileDefaultAndNamed(t *testing.T) {
	dir := t.TempDir()
	cfg, jar, err := PathsForProfile(dir, DefaultProfile)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != filepath.Join(dir, "config.json") || jar != filepath.Join(dir, "cookies.txt") {
		t.Fatalf("default paths %s %s", cfg, jar)
	}
	cfg, jar, err = PathsForProfile(dir, "dev")
	if err != nil {
		t.Fatal(err)
	}
	wantCfg := filepath.Join(dir, "profiles", "dev", "config.json")
	wantJar := filepath.Join(dir, "profiles", "dev", "cookies.txt")
	if cfg != wantCfg || jar != wantJar {
		t.Fatalf("named paths %s %s", cfg, jar)
	}
}

func TestSetCurrentListAndResolve(t *testing.T) {
	dir := t.TempDir()
	names, err := ListProfiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != DefaultProfile {
		t.Fatalf("list=%v", names)
	}
	cur, err := CurrentProfile(dir)
	if err != nil || cur != DefaultProfile {
		t.Fatalf("current=%s err=%v", cur, err)
	}
	if err := SetCurrent(dir, "prod"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "profiles", "prod")); err != nil {
		t.Fatal(err)
	}
	cur, err = CurrentProfile(dir)
	if err != nil || cur != "prod" {
		t.Fatalf("current=%s err=%v", cur, err)
	}
	names, err = ListProfiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != DefaultProfile || names[1] != "prod" {
		t.Fatalf("list=%v", names)
	}
	got, err := ResolveProfileName("dev", "ignored", dir)
	if err != nil || got != "dev" {
		t.Fatalf("flag resolve %s %v", got, err)
	}
	got, err = ResolveProfileName("", "staging", dir)
	if err != nil || got != "staging" {
		t.Fatalf("env resolve %s %v", got, err)
	}
	got, err = ResolveProfileName("", "", dir)
	if err != nil || got != "prod" {
		t.Fatalf("meta resolve %s %v", got, err)
	}
}
