package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/output"
	"plumelog-cli/internal/risk"
)

type skillInstallItem struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
	Mode string `json:"mode"` // copy | symlink
}

type skillSkipItem struct {
	Name   string `json:"name"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason"`
}

func defaultSkillsInstallDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".agents", "skills")
	}
	return filepath.Join(home, ".agents", "skills")
}

// discoverInstallableSkills returns plumelog-* dirs under root that contain SKILL.md.
// If filter is non-empty, only those names (must still exist and have SKILL.md).
func discoverInstallableSkills(root string, filter []string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("skills source %s: %w", root, err)
	}
	want := map[string]bool{}
	for _, n := range filter {
		n = strings.TrimSpace(n)
		if n != "" {
			want[n] = true
		}
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "plumelog-") {
			continue
		}
		if len(want) > 0 && !want[e.Name()] {
			continue
		}
		skillMD := filepath.Join(root, e.Name(), "SKILL.md")
		if st, err := os.Stat(skillMD); err != nil || st.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(want) > 0 {
		found := map[string]bool{}
		for _, n := range names {
			found[n] = true
		}
		var missing []string
		for n := range want {
			if !found[n] {
				missing = append(missing, n)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("skill(s) not found or missing SKILL.md: %s", strings.Join(missing, ", "))
		}
	}
	return names, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDirRecursive(src, dst string) error {
	src = filepath.Clean(src)
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
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		return copyFile(path, target, mode)
	})
}

func installOneSkill(src, dst string, symlink, force, dryRun bool) (installed *skillInstallItem, skipped *skillSkipItem, err error) {
	mode := "copy"
	if symlink {
		mode = "symlink"
	}
	item := &skillInstallItem{Name: filepath.Base(src), From: src, To: dst, Mode: mode}

	if _, err := os.Lstat(dst); err == nil {
		if !force {
			return nil, &skillSkipItem{
				Name:   item.Name,
				From:   src,
				To:     dst,
				Reason: "exists",
			}, nil
		}
		if dryRun {
			return item, nil, nil
		}
		if err := os.RemoveAll(dst); err != nil {
			return nil, nil, fmt.Errorf("remove existing %s: %w", dst, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}

	if dryRun {
		return item, nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, nil, err
	}
	if symlink {
		absSrc, err := filepath.Abs(src)
		if err != nil {
			return nil, nil, err
		}
		if err := os.Symlink(absSrc, dst); err != nil {
			return nil, nil, fmt.Errorf("symlink %s -> %s: %w", dst, absSrc, err)
		}
		return item, nil, nil
	}
	if err := copyDirRecursive(src, dst); err != nil {
		return nil, nil, fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	return item, nil, nil
}

func installSkills(srcRoot, targetDir string, filter []string, symlink, force, dryRun bool, warn io.Writer) (map[string]any, error) {
	names, err := discoverInstallableSkills(srcRoot, filter)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no installable plumelog-* skills with SKILL.md under %s", srcRoot)
	}

	installed := []skillInstallItem{}
	skipped := []skillSkipItem{}

	for _, name := range names {
		from := filepath.Join(srcRoot, name)
		to := filepath.Join(targetDir, name)
		ins, sk, err := installOneSkill(from, to, symlink, force, dryRun)
		if err != nil {
			return nil, err
		}
		if sk != nil {
			skipped = append(skipped, *sk)
		}
		if ins != nil {
			installed = append(installed, *ins)
		}
	}

	if len(skipped) > 0 && warn != nil {
		skippedNames := make([]string, 0, len(skipped))
		for _, s := range skipped {
			skippedNames = append(skippedNames, s.Name)
		}
		fmt.Fprintf(warn, "warning: skills install skipped %d existing skill(s) under %s: %s (rerun with --force to replace them)\n",
			len(skipped), targetDir, strings.Join(skippedNames, ", "))
	}

	result := map[string]any{
		"installed":       installed,
		"skipped":         skipped,
		"installed_count": len(installed),
		"skipped_count":   len(skipped),
		"target_dir":      targetDir,
		"source_dir":      srcRoot,
	}
	if len(skipped) > 0 {
		result["hint"] = "skipped skills already exist at the target dir; rerun with --force to replace them"
	}
	return result, nil
}

func (a *App) newSkillsInstallCmd() *cobra.Command {
	var (
		installDir string
		names      []string
		symlink    bool
		force      bool
	)
	cmd := &cobra.Command{
		Use: "install",
		Long: `Install skills from the repo skills/plumelog-* directories into the user's
agent skills directory (default: ~/.agents/skills on Unix, %USERPROFILE%\.agents\skills on Windows)
so agents can discover them.

Same layout as yunxiao "skills install". Default mode is recursive copy; use --symlink
to link instead. Existing target skill dirs are skipped unless --force.

Risk: write`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.format = firstNonEmpty(a.format, "json")
			targetDir := installDir
			if targetDir == "" {
				targetDir = defaultSkillsInstallDir()
			}
			absTarget, err := filepath.Abs(targetDir)
			if err != nil {
				return a.fail(cmd, risk.Write, err)
			}
			result, err := installSkills(skillsRoot(), absTarget, names, symlink, force, a.dryRun, cmd.ErrOrStderr())
			if err != nil {
				return a.fail(cmd, risk.Write, err)
			}
			if a.dryRun {
				env := output.Envelope{
					OK:     true,
					DryRun: true,
					Risk:   risk.Write,
					Result: output.MustResult(result),
				}
				return a.write(cmd, env)
			}
			return a.ok(cmd, result)
		},
	}
	mark(cmd, risk.Write, "Install companion skills into the agent skills directory")
	cmd.Flags().StringVar(&installDir, "dir", "", "install root (default: ~/.agents/skills)")
	cmd.Flags().StringArrayVar(&names, "skill", nil, "install only named skill (repeatable)")
	cmd.Flags().BoolVar(&symlink, "symlink", false, "symlink instead of copy")
	cmd.Flags().BoolVar(&force, "force", false, "replace existing target skill dirs")
	return cmd
}