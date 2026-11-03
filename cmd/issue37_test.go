package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIssue37NamedProfilesIsolateSessions(t *testing.T) {
	home := t.TempDir()
	setConfigHome(t, home)

	out, err := runHome(t, home, "config", "profile", "set", "dev", "base_url", "http://192.168.100.115:8891")
	if err != nil {
		t.Fatal(err, out)
	}
	out, err = runHome(t, home, "config", "profile", "set", "prod", "base_url", "http://plumelog.example.com")
	if err != nil {
		t.Fatal(err, out)
	}

	devCfg := filepath.Join(home, "plumelog", "profiles", "dev", "config.json")
	prodCfg := filepath.Join(home, "plumelog", "profiles", "prod", "config.json")
	devJar := filepath.Join(home, "plumelog", "profiles", "dev", "cookies.txt")
	prodJar := filepath.Join(home, "plumelog", "profiles", "prod", "cookies.txt")
	if _, err := os.Stat(devCfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(prodCfg); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(devJar, []byte("# Netscape HTTP Cookie File\n# dev-session\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prodJar, []byte("# Netscape HTTP Cookie File\n# prod-session\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	devPath, err := runHome(t, home, "--profile", "dev", "config", "path")
	if err != nil {
		t.Fatal(err, devPath)
	}
	prodPath, err := runHome(t, home, "--profile", "prod", "config", "path")
	if err != nil {
		t.Fatal(err, prodPath)
	}
	var devEnv, prodEnv map[string]any
	if err := json.Unmarshal([]byte(devPath), &devEnv); err != nil {
		t.Fatal(err, devPath)
	}
	if err := json.Unmarshal([]byte(prodPath), &prodEnv); err != nil {
		t.Fatal(err, prodPath)
	}
	devRes := devEnv["result"].(map[string]any)
	prodRes := prodEnv["result"].(map[string]any)
	if devRes["profile"] != "dev" || prodRes["profile"] != "prod" {
		t.Fatalf("profiles %v %v", devRes["profile"], prodRes["profile"])
	}
	if devRes["config_path"] == prodRes["config_path"] || devRes["cookie_path"] == prodRes["cookie_path"] {
		t.Fatalf("paths must differ: %s vs %s", devPath, prodPath)
	}
	if !strings.Contains(devRes["cookie_path"].(string), filepath.Join("profiles", "dev")) {
		t.Fatalf("dev cookie path %v", devRes["cookie_path"])
	}
	if !strings.Contains(prodRes["cookie_path"].(string), filepath.Join("profiles", "prod")) {
		t.Fatalf("prod cookie path %v", prodRes["cookie_path"])
	}

	// Overwrite only the prod jar; dev jar content must remain.
	if err := os.WriteFile(prodJar, []byte("# Netscape HTTP Cookie File\n# prod-overwritten\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	devBytes, err := os.ReadFile(devJar)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(devBytes, []byte("dev-session")) {
		t.Fatalf("dev jar overwritten: %s", devBytes)
	}
	prodBytes, err := os.ReadFile(prodJar)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(prodBytes, []byte("prod-overwritten")) {
		t.Fatalf("prod jar missing update: %s", prodBytes)
	}

	devShow, err := runHome(t, home, "--profile", "dev", "config", "show")
	if err != nil {
		t.Fatal(err, devShow)
	}
	prodShow, err := runHome(t, home, "--profile", "prod", "config", "show")
	if err != nil {
		t.Fatal(err, prodShow)
	}
	if !strings.Contains(devShow, "192.168.100.115:8891") || strings.Contains(devShow, "plumelog.example.com") {
		t.Fatalf("dev show leaked/wrong base: %s", devShow)
	}
	if !strings.Contains(prodShow, "plumelog.example.com") || strings.Contains(prodShow, "192.168.100.115:8891") {
		t.Fatalf("prod show leaked/wrong base: %s", prodShow)
	}
}

func TestIssue37DefaultProfileCompatible(t *testing.T) {
	home := t.TempDir()
	setConfigHome(t, home)

	out, err := runHome(t, home, "config", "set", "base_url", "http://127.0.0.1:8891")
	if err != nil {
		t.Fatal(err, out)
	}
	legacyCfg := filepath.Join(home, "plumelog", "config.json")
	legacyJar := filepath.Join(home, "plumelog", "cookies.txt")
	if _, err := os.Stat(legacyCfg); err != nil {
		t.Fatalf("default should use flat config.json: %v", err)
	}
	pathOut, err := runHome(t, home, "config", "path")
	if err != nil {
		t.Fatal(err, pathOut)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(pathOut), &env); err != nil {
		t.Fatal(err, pathOut)
	}
	res := env["result"].(map[string]any)
	if res["profile"] != "default" {
		t.Fatalf("profile=%v", res["profile"])
	}
	if res["config_path"] != legacyCfg {
		t.Fatalf("config_path=%v want %s", res["config_path"], legacyCfg)
	}
	if res["cookie_path"] != legacyJar {
		t.Fatalf("cookie_path=%v want %s", res["cookie_path"], legacyJar)
	}
	if _, err := os.Stat(filepath.Join(home, "plumelog", "profiles")); !os.IsNotExist(err) {
		t.Fatalf("default-only should not create profiles/ yet: %v", err)
	}
}

func TestIssue37ProfileUseAndList(t *testing.T) {
	home := t.TempDir()
	setConfigHome(t, home)

	out, err := runHome(t, home, "config", "profile", "use", "dev")
	if err != nil {
		t.Fatal(err, out)
	}
	listOut, err := runHome(t, home, "config", "profile", "list")
	if err != nil {
		t.Fatal(err, listOut)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(listOut), &env); err != nil {
		t.Fatal(err, listOut)
	}
	res := env["result"].(map[string]any)
	if res["current"] != "dev" {
		t.Fatalf("current=%v", res["current"])
	}
	// Without --profile, current from profiles.json should apply.
	pathOut, err := runHome(t, home, "config", "path")
	if err != nil {
		t.Fatal(err, pathOut)
	}
	var pathEnv map[string]any
	if err := json.Unmarshal([]byte(pathOut), &pathEnv); err != nil {
		t.Fatal(err, pathOut)
	}
	if pathEnv["result"].(map[string]any)["profile"] != "dev" {
		t.Fatalf("path after use: %s", pathOut)
	}
}

func TestIssue37ProfileRejectsSecrets(t *testing.T) {
	home := t.TempDir()
	setConfigHome(t, home)
	stdout, stderr, err := runHomeStreams(t, home, "config", "profile", "set", "dev", "password", "s3cret")
	if err == nil {
		t.Fatal("expected refusal")
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "refusing") || strings.Contains(stderr, "s3cret") {
		t.Fatalf("stderr=%s", stderr)
	}
}

func setConfigHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("APPDATA", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("PLUMELOG_PROFILE", "")
	t.Setenv("PLUMELOG_BASE_URL", "")
	t.Setenv("PLUMELOG_PATH_PREFIX", "")
	t.Setenv("PLUMELOG_AUTO_LOGIN", "")
	if runtime.GOOS != "windows" {
		t.Setenv("HOME", home)
	}
}

func runHome(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	stdout, stderr, err := runHomeStreams(t, home, args...)
	if err != nil && strings.TrimSpace(stderr) != "" {
		return stderr, err
	}
	return stdout, err
}

func runHomeStreams(t *testing.T, home string, args ...string) (string, string, error) {
	t.Helper()
	setConfigHome(t, home)
	root := NewRoot()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	full := append([]string{"--format", "json"}, args...)
	root.SetArgs(full)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}
