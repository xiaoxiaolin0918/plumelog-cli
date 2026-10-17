package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func falseProbe(string) bool { return false }

func TestRestoreMSYSMangledAPIPath(t *testing.T) {
	probeGitRoot := func(root string) bool { return root == "C:/Program Files/Git" }
	probeMsysUnderPlumelog := func(root string) bool { return root == "D:/plumelog/msys64" }

	cases := []struct {
		name     string
		arg      string
		envSig   bool
		probe    func(string) bool
		want     string
		wantBack bool
	}{
		{
			name:     "clientQuery with env signal",
			arg:      "C:/Program Files/Git/clientQuery",
			envSig:   true,
			probe:    falseProbe,
			want:     "/clientQuery",
			wantBack: true,
		},
		{
			name:     "plumelog prefix with env signal",
			arg:      "C:/Program Files/Git/plumelog/login",
			envSig:   true,
			probe:    falseProbe,
			want:     "/plumelog/login",
			wantBack: true,
		},
		{
			name:     "query string survives",
			arg:      "C:/Program Files/Git/clientQuery?size=20",
			envSig:   true,
			probe:    falseProbe,
			want:     "/clientQuery?size=20",
			wantBack: true,
		},
		{
			name:     "lowercase drive",
			arg:      "c:/git/getRunModel",
			envSig:   true,
			probe:    falseProbe,
			want:     "/getRunModel",
			wantBack: true,
		},
		{
			name:     "probe-validated root without env signal",
			arg:      "C:/Program Files/Git/clientQuery",
			envSig:   false,
			probe:    probeGitRoot,
			want:     "/clientQuery",
			wantBack: true,
		},
		{
			name:     "probe picks real root when it contains plumelog dir",
			arg:      "D:/plumelog/msys64/plumelog/login",
			envSig:   false,
			probe:    probeMsysUnderPlumelog,
			want:     "/plumelog/login",
			wantBack: true,
		},
		{
			name:     "drive-shaped but no signal and no valid probe: unchanged",
			arg:      "C:/Program Files/Git/clientQuery",
			envSig:   false,
			probe:    falseProbe,
			wantBack: false,
		},
		{
			name:     "plain API path unchanged",
			arg:      "/clientQuery",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
		{
			name:     "full URL unchanged",
			arg:      "http://localhost:8891/clientQuery",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
		{
			name:     "drive path without known marker unchanged under env signal",
			arg:      "C:/Program Files/Git/usr/bin/bash.exe",
			envSig:   true,
			probe:    falseProbe,
			wantBack: false,
		},
		{
			name:     "/query must not steal /queryAppName prefix",
			arg:      "C:/Program Files/Git/queryAppName",
			envSig:   true,
			probe:    falseProbe,
			want:     "/queryAppName",
			wantBack: true,
		},
		{
			name:     "questionnaire is not /query",
			arg:      "C:/Program Files/Git/questionnaire",
			envSig:   true,
			probe:    falseProbe,
			wantBack: false,
		},
		{
			name:     "empty unchanged",
			arg:      "",
			envSig:   true,
			probe:    probeGitRoot,
			wantBack: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := restoreMSYSMangledAPIPath(tc.arg, tc.envSig, tc.probe)
			if ok != tc.wantBack {
				t.Fatalf("ok=%v want %v (arg %q got %q)", ok, tc.wantBack, tc.arg, got)
			}
			if ok && got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if !ok && got != "" {
				t.Fatalf("not-restored must return empty, got %q", got)
			}
		})
	}
}

func TestUnmangleAPIPathArgDoubleSlashCollapse(t *testing.T) {
	got, note := unmangleAPIPathArg("//clientQuery")
	if got != "/clientQuery" {
		t.Fatalf("got %q", got)
	}
	if note == "" {
		t.Fatal("expected a note for the collapse")
	}
}

func TestUnmangleAPIPathArgKillSwitch(t *testing.T) {
	t.Setenv(envAPIUnmangleOff, "1")
	t.Setenv("MSYSTEM", "MINGW64")
	got, note := unmangleAPIPathArg("C:/Program Files/Git/clientQuery")
	if got != "C:/Program Files/Git/clientQuery" || note != "" {
		t.Fatalf("kill switch must leave arg untouched, got %q note %q", got, note)
	}
}

func TestAPICmdUnmanglesMSYSPathEndToEnd(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = io.WriteString(w, `{"logs":[]}`)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("MSYSTEM", "MINGW64")
	t.Setenv("MSYS", "")
	t.Setenv(envAPIUnmangleOff, "")

	var noteBuf bytes.Buffer
	prev := msysNoteOut
	msysNoteOut = &noteBuf
	t.Cleanup(func() { msysNoteOut = prev })

	dir := t.TempDir()
	out, err := runIn(t, dir, "api", "POST", "C:/Program Files/Git/getRunModel", "--base-url", srv.URL)
	if err != nil {
		t.Fatal(err, out)
	}
	if gotPath != "/getRunModel" {
		t.Fatalf("server saw path %q, want /getRunModel", gotPath)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err, out)
	}
	if env["ok"] != true {
		t.Fatalf("envelope: %s", out)
	}
	if !strings.Contains(noteBuf.String(), "note:") || !strings.Contains(noteBuf.String(), "MSYS_NO_PATHCONV") {
		t.Fatalf("stderr note missing: %q", noteBuf.String())
	}
}

func TestProbeMSYSRootOnDisk(t *testing.T) {
	root := t.TempDir()
	if probeMSYSRootOnDisk(root) {
		t.Fatal("empty temp dir must not look like MSYS")
	}
	if err := os.MkdirAll(filepath.Join(root, "usr", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !probeMSYSRootOnDisk(root) {
		t.Fatal("usr/bin should count as MSYS root")
	}
}
