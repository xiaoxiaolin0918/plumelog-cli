package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/config"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/version"
)

func TestShouldSkipBackgroundUpdate(t *testing.T) {
	root := NewRoot()
	upd, _, err := root.Find([]string{"update"})
	if err != nil {
		t.Fatal(err)
	}
	if !shouldSkipBackgroundUpdate(upd) {
		t.Fatal("update should skip")
	}
	doc, _, err := root.Find([]string{"doctor"})
	if err != nil {
		t.Fatal(err)
	}
	if shouldSkipBackgroundUpdate(doc) {
		t.Fatal("doctor should not skip")
	}
	ver, _, err := root.Find([]string{"version"})
	if err != nil {
		t.Fatal(err)
	}
	if shouldSkipBackgroundUpdate(ver) {
		t.Fatal("version should not skip")
	}
	sk, _, err := root.Find([]string{"skills", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if !shouldSkipBackgroundUpdate(sk) {
		t.Fatal("skills should skip")
	}
}

func TestUpdateCheckCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	c := updateCheckCache{CheckedAt: time.Now().UTC(), Latest: "9.9.9", Status: "outdated"}
	saveUpdateCheckCache(c)
	got, ok := loadUpdateCheckCache()
	if !ok || got.Latest != "9.9.9" || got.Status != "outdated" {
		t.Fatalf("%v ok=%v", got, ok)
	}
}

func boolPtr(v bool) *bool { return &v }

func TestResolveUpdateFlagsDefaultsAndEnv(t *testing.T) {
	t.Setenv("PLUMELOG_CHECK_UPDATE", "")
	t.Setenv("PLUMELOG_AUTO_UPDATE", "")
	a := &App{}
	a.resolveUpdateFlags()
	if !a.checkUpdateEnabled || !a.autoUpdateEnabled {
		t.Fatal("default on")
	}

	t.Setenv("PLUMELOG_AUTO_UPDATE", "0")
	a = &App{}
	a.resolveUpdateFlags()
	if !a.checkUpdateEnabled || a.autoUpdateEnabled {
		t.Fatal("env auto=0 keeps check, disables auto")
	}

	t.Setenv("PLUMELOG_CHECK_UPDATE", "0")
	t.Setenv("PLUMELOG_AUTO_UPDATE", "")
	a = &App{}
	a.resolveUpdateFlags()
	if a.checkUpdateEnabled || a.autoUpdateEnabled {
		t.Fatal("env check=0 disables hook including auto")
	}

	t.Setenv("PLUMELOG_CHECK_UPDATE", "")
	t.Setenv("PLUMELOG_AUTO_UPDATE", "")
	a = &App{cfg: config.File{AutoUpdate: boolPtr(false)}}
	a.resolveUpdateFlags()
	if !a.checkUpdateEnabled || a.autoUpdateEnabled {
		t.Fatal("config auto_update false keeps check")
	}

	a = &App{cfg: config.File{CheckUpdate: boolPtr(false)}}
	a.resolveUpdateFlags()
	if a.checkUpdateEnabled || a.autoUpdateEnabled {
		t.Fatal("config check_update false disables hook including auto")
	}

	a = &App{flagNoAutoUpdate: true}
	a.resolveUpdateFlags()
	if !a.checkUpdateEnabled || a.autoUpdateEnabled {
		t.Fatal("--no-auto-update keeps check")
	}

	a = &App{flagNoCheckUpdate: true, flagAutoUpdate: true}
	a.resolveUpdateFlags()
	if a.checkUpdateEnabled || a.autoUpdateEnabled {
		t.Fatal("--no-check-update wins over --auto-update")
	}

	a = &App{flagNoAutoUpdate: true, flagAutoUpdate: true}
	a.resolveUpdateFlags()
	if !a.checkUpdateEnabled || a.autoUpdateEnabled {
		t.Fatal("--no-auto-update wins over --auto-update")
	}

	t.Setenv("PLUMELOG_AUTO_UPDATE", "0")
	a = &App{flagAutoUpdate: true}
	a.resolveUpdateFlags()
	if !a.checkUpdateEnabled || !a.autoUpdateEnabled {
		t.Fatal("--auto-update overrides env 0")
	}
}

func TestBackgroundCheckHintAndSilentAutoUpdate(t *testing.T) {
	payload := []byte("fake-plumelog-silent-update")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	asset := "plumelog_linux_amd64"
	checksums := hexSum + "  " + asset + "\n"

	mux := http.NewServeMux()
	var binHits int
	var srvURL string
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v9.9.9",
			"html_url": "http://example.test/release",
			"assets": []map[string]any{
				{"name": asset, "browser_download_url": srvURL + "/bin", "size": len(payload)},
				{"name": "checksums.txt", "browser_download_url": srvURL + "/sum", "size": len(checksums)},
			},
		})
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		binHits++
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/sum", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	oldClient, oldURL, oldExe, oldOS, oldArch, oldQuick := updateHTTPClient, updateAPIURL, updateExecutable, updateGOOS, updateGOARCH, updateCheckHTTPClient
	defer func() {
		updateHTTPClient, updateAPIURL, updateExecutable, updateGOOS, updateGOARCH, updateCheckHTTPClient = oldClient, oldURL, oldExe, oldOS, oldArch, oldQuick
	}()
	updateHTTPClient = srv.Client()
	updateCheckHTTPClient = srv.Client()
	updateAPIURL = srv.URL + "/releases/latest"
	updateGOOS, updateGOARCH = "linux", "amd64"

	home := t.TempDir()
	t.Setenv("APPDATA", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("PLUMELOG_CHECK_UPDATE", "1")
	t.Setenv("PLUMELOG_AUTO_UPDATE", "0")

	dir := t.TempDir()
	exe := filepath.Join(dir, "plumelog")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	updateExecutable = func() (string, error) { return exe, nil }

	root := NewRoot()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version check: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), version.Version) || strings.Contains(stdout.String(), "newer version") {
		t.Fatalf("hint leaked to stdout: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "newer version 9.9.9") || !strings.Contains(stderr.String(), "plumelog update --yes") {
		t.Fatalf("missing stderr hint: %s", stderr.String())
	}
	if binHits != 0 {
		t.Fatal("check-update must not download")
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("binary replaced without auto-update: %q", got)
	}

	t.Setenv("PLUMELOG_AUTO_UPDATE", "1")
	_ = os.Remove(filepath.Join(home, "plumelog", "update-check-cache.json"))
	stdout.Reset()
	stderr.Reset()
	root = NewRoot()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version auto: %v stderr=%s", err, stderr.String())
	}
	if binHits != 1 {
		t.Fatalf("auto-update downloads: bin=%d", binHits)
	}
	got, err = os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("binary not replaced: %q", got)
	}
	if strings.Contains(stdout.String(), "auto-updated") {
		t.Fatalf("auto-update leaked to stdout: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "auto-updated to 9.9.9") {
		t.Fatalf("missing auto-update stderr: %s", stderr.String())
	}
}
