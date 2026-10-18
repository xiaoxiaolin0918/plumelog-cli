package credstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	err := Save(dir, Cred{Username: "alice", Password: "s3cret-roundtrip", AutoLogin: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "s3cret-roundtrip") {
		t.Fatal("plaintext password landed in login.cred")
	}
	if !strings.Contains(string(raw), `"auto_login": true`) {
		t.Fatalf("missing auto_login: %s", raw)
	}
	got, ok, err := Load(dir)
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if got.Username != "alice" || got.Password != "s3cret-roundtrip" || !got.AutoLogin {
		t.Fatalf("%+v", got)
	}
	u, auto, ok := PeekUsernameAutoLogin(dir)
	if !ok || u != "alice" || !auto {
		t.Fatalf("peek %q %v %v", u, auto, ok)
	}
}

func TestLoadMissing(t *testing.T) {
	_, ok, err := Load(t.TempDir())
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Cred{Username: "u", Password: "p", AutoLogin: false}); err != nil {
		t.Fatal(err)
	}
	if err := Clear(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(Path(dir)); !os.IsNotExist(err) {
		t.Fatalf("cred still present: %v", err)
	}
	if err := Clear(dir); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Cred{Username: "", Password: "p"}); err == nil {
		t.Fatal("expected username error")
	}
	if err := Save(dir, Cred{Username: "u", Password: ""}); err == nil {
		t.Fatal("expected password error")
	}
}

func TestDirFromConfigPath(t *testing.T) {
	got := DirFromConfigPath(filepath.Join("a", "b", "config.json"))
	if got != filepath.Join("a", "b") {
		t.Fatalf("%s", got)
	}
}
