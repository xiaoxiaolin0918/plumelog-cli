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
)

func TestReleaseAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
		ok                 bool
	}{
		{"windows", "amd64", "plumelog_windows_amd64.exe", true},
		{"linux", "amd64", "plumelog_linux_amd64", true},
		{"linux", "arm64", "plumelog_linux_arm64", true},
		{"darwin", "amd64", "plumelog_darwin_amd64", true},
		{"darwin", "arm64", "plumelog_darwin_arm64", true},
		{"windows", "arm64", "", false},
		{"plan9", "amd64", "", false},
	}
	for _, tc := range cases {
		got, err := releaseAssetName(tc.goos, tc.goarch)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Fatalf("%s/%s: got %q err=%v want %q", tc.goos, tc.goarch, got, err, tc.want)
			}
		} else if err == nil {
			t.Fatalf("%s/%s: expected error", tc.goos, tc.goarch)
		}
	}
}

func TestParseChecksums(t *testing.T) {
	text := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899  plumelog_linux_amd64\n" +
		"11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff *plumelog_windows_amd64.exe\n" +
		"bad line\n"
	m := parseChecksums(text)
	if m["plumelog_linux_amd64"] != "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899" {
		t.Fatalf("linux: %#v", m)
	}
	if m["plumelog_windows_amd64.exe"] != "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff" {
		t.Fatalf("windows: %#v", m)
	}
}

func TestStatusVsLatest(t *testing.T) {
	if statusVsLatest("0.1.4", "v0.1.4") != "up_to_date" {
		t.Fatal("equal")
	}
	if statusVsLatest("0.1.3", "v0.1.4") != "outdated" {
		t.Fatal("outdated")
	}
	if statusVsLatest("0.2.0", "v0.1.4") != "ahead" {
		t.Fatal("ahead")
	}
}

func TestUpdateCheckAndApply(t *testing.T) {
	payload := []byte("fake-plumelog-binary-bytes")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	asset := "plumelog_linux_amd64"
	checksums := hexSum + "  " + asset + "\n"

	mux := http.NewServeMux()
	var binHits, sumHits int
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
		sumHits++
		_, _ = w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	oldClient, oldURL, oldExe, oldOS, oldArch := updateHTTPClient, updateAPIURL, updateExecutable, updateGOOS, updateGOARCH
	defer func() {
		updateHTTPClient, updateAPIURL, updateExecutable, updateGOOS, updateGOARCH = oldClient, oldURL, oldExe, oldOS, oldArch
	}()
	updateHTTPClient = srv.Client()
	updateAPIURL = srv.URL + "/releases/latest"
	updateGOOS, updateGOARCH = "linux", "amd64"

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
	cfg := filepath.Join(dir, "config.json")
	jar := filepath.Join(dir, "cookies.txt")
	root.SetArgs([]string{"--config", cfg, "--cookie-jar", jar, "update", "--check"})
	if err := root.Execute(); err != nil {
		t.Fatalf("check: %v stderr=%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "outdated") {
		t.Fatalf("check stdout=%s", stdout.String())
	}
	if binHits != 0 || sumHits != 0 {
		t.Fatalf("--check must not download: bin=%d sum=%d", binHits, sumHits)
	}

	stdout.Reset()
	stderr.Reset()
	root = NewRoot()
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs([]string{"--config", cfg, "--cookie-jar", jar, "update", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("apply: %v stderr=%s", err, stderr.String())
	}
	if binHits != 1 || sumHits != 1 {
		t.Fatalf("apply downloads: bin=%d sum=%d", binHits, sumHits)
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("binary not replaced: %q", got)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &env); err != nil {
		t.Fatalf("json: %v stdout=%s", err, stdout.String())
	}
	res, _ := env["result"].(map[string]any)
	if res["applied"] != true {
		t.Fatalf("result=%v", res)
	}
}