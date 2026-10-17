// Package skillfs embeds the plumelog-* agent skill trees into the CLI binary
// so install/update work without a git checkout next to the executable.
package skillfs

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"plumelog-cli/internal/version"
)

//go:embed all:plumelog-admin all:plumelog-alarm all:plumelog-api all:plumelog-app all:plumelog-query all:plumelog-shared all:plumelog-trace
var FS embed.FS

// Names returns embedded plumelog-* skill directory names (sorted by Walk order filter).
func Names() []string {
	var names []string
	_ = fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == "." {
			return nil
		}
		if strings.Count(path, string(os.PathSeparator)) == 0 && strings.Count(path, "/") == 0 && strings.HasPrefix(path, "plumelog-") {
			names = append(names, path)
			return fs.SkipDir
		}
		// embed always uses /
		if !strings.Contains(path, "/") && strings.HasPrefix(path, "plumelog-") {
			names = append(names, path)
			return fs.SkipDir
		}
		return nil
	})
	return names
}

// Materialize extracts embedded skills into the user cache (versioned) and
// returns that directory. Idempotent when the marker file already matches.
func Materialize() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil || cache == "" {
		cache = os.TempDir()
	}
	dest := filepath.Join(cache, "plumelog-cli", "bundled-skills-"+version.Version)
	marker := filepath.Join(dest, ".bundled-version")
	if b, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(b)) == version.Version {
		if st, err := os.Stat(filepath.Join(dest, "plumelog-shared", "SKILL.md")); err == nil && !st.IsDir() {
			return dest, nil
		}
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dest), "skills-extract-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := extractAll(FS, tmp); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, ".bundled-version"), []byte(version.Version+"\n"), 0o644); err != nil {
		return "", err
	}
	_ = os.RemoveAll(dest)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		// Cross-device fallback: copy then remove tmp (defer also removes)
		if err2 := copyDir(tmp, dest); err2 != nil {
			return "", fmt.Errorf("materialize skills: rename: %v; copy: %w", err, err2)
		}
	}
	return dest, nil
}

func extractAll(efs embed.FS, dest string) error {
	return fs.WalkDir(efs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(dest, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := efs.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}