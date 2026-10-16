package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDirFor(t *testing.T) {
	win, err := dirFor("windows", filepath.Join("C:", "Users", "a", "AppData", "Roaming"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("C:", "Users", "a", "AppData", "Roaming", "plumelog")
	if win != want {
		t.Fatalf("windows %s", win)
	}
	xdg, err := dirFor("linux", "", "/tmp/xdg", "/home/dev")
	if err != nil {
		t.Fatal(err)
	}
	if xdg != filepath.Join("/tmp/xdg", "plumelog") {
		t.Fatalf("xdg %s", xdg)
	}
	home, err := dirFor("linux", "", "", "/home/dev")
	if err != nil {
		t.Fatal(err)
	}
	if home != filepath.Join("/home/dev", ".config", "plumelog") {
		t.Fatalf("home %s", home)
	}
}

func TestSaveModeAndRejectsBadPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	s := "/plumelog"
	if err := Save(path, File{BaseURL: "http://127.0.0.1:8891", PathPrefix: &s}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
	loaded, ok, err := Load(path)
	if err != nil || !ok || loaded.BaseURL != "http://127.0.0.1:8891" || loaded.PathPrefix == nil || *loaded.PathPrefix != "/plumelog" {
		t.Fatalf("%+v %v %v", loaded, ok, err)
	}
	bad := "/plumelogServer"
	if err := Save(path, File{PathPrefix: &bad}); err == nil {
		t.Fatal("expected prefix error")
	}
	if !SecretKey("admin_password") || SecretKey("base_url") {
		t.Fatal("secret key classification")
	}
}
