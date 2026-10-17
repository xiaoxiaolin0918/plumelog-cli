package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/risk"
	skillfs "plumelog-cli/skills"
)

var skillVersionRE = regexp.MustCompile(`(?m)^version:\s*["']?([^\s"']+)["']?\s*$`)

// findDiskSkills locates an on-disk skills/ tree (next to binary, source, or cwd).
func findDiskSkills() string {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "skills"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "skills"))
	}
	_, file, _, _ := runtime.Caller(0)
	candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "skills"))
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "skills"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			// Require at least one plumelog-* skill so an empty placeholder dir loses to embed.
			if hasPlumelogSkill(c) {
				abs, err := filepath.Abs(c)
				if err == nil {
					return abs
				}
				return c
			}
		}
	}
	return ""
}

func hasPlumelogSkill(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "plumelog-") {
			md := filepath.Join(root, e.Name(), "SKILL.md")
			if st, err := os.Stat(md); err == nil && !st.IsDir() {
				return true
			}
		}
	}
	return false
}

// skillsRoot finds the bundled skills/ directory (disk preferred, else embedded cache).
func skillsRoot() string {
	if disk := findDiskSkills(); disk != "" {
		return disk
	}
	if dest, err := skillfs.Materialize(); err == nil {
		return dest
	}
	return "skills"
}

func skillsSourceMeta() (root string, source string) {
	if disk := findDiskSkills(); disk != "" {
		return disk, "disk"
	}
	if dest, err := skillfs.Materialize(); err == nil {
		return dest, "embedded"
	}
	return "skills", "missing"
}

func isSkillsCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "skills" {
			return true
		}
	}
	return false
}

func readSkillVersion(skillMD string) string {
	b, err := os.ReadFile(skillMD)
	if err != nil {
		return ""
	}
	m := skillVersionRE.FindSubmatch(b)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func (a *App) newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "List, read, install, or update bundled agent skill content",
		RunE:  helpOnly,
	}

	var listCheck bool
	list := &cobra.Command{
		Use: "list",
		Long: `List bundled plumelog-* skills.

With --check, also compare each skill's frontmatter version against the copy
installed under the agent skills directory (default ~/.agents/skills).
Status uses semver direction of installed relative to bundled:
equal, outdated, ahead, or invalid; plus missing / unknown when a side has
no readable version.

Risk: read`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.format = firstNonEmpty(a.format, "json")
			root, source := skillsSourceMeta()
			entries, err := os.ReadDir(root)
			if err != nil {
				return a.fail(cmd, risk.Read, fmt.Errorf("skills dir %s: %w", root, err))
			}
			var names []string
			for _, e := range entries {
				if e.IsDir() && strings.HasPrefix(e.Name(), "plumelog-") {
					md := filepath.Join(root, e.Name(), "SKILL.md")
					if st, err := os.Stat(md); err == nil && !st.IsDir() {
						names = append(names, e.Name())
					}
				}
			}
			result := map[string]any{"skills": names, "skills_dir": root, "source": source}
			if listCheck {
				installDir := defaultSkillsInstallDir()
				var statuses []map[string]any
				for _, name := range names {
					bundledVer := readSkillVersion(filepath.Join(root, name, "SKILL.md"))
					instMD := filepath.Join(installDir, name, "SKILL.md")
					instVer := readSkillVersion(instMD)
					status := "missing"
					switch {
					case instVer == "" && bundledVer != "":
						if _, err := os.Stat(instMD); err == nil {
							status = "unknown"
						} else {
							status = "missing"
						}
					case instVer == "" && bundledVer == "":
						if _, err := os.Stat(instMD); err == nil {
							status = string(RelEqual)
						} else {
							status = "missing"
						}
					default:
						status = string(CompareSemver(instVer, bundledVer))
					}
					statuses = append(statuses, map[string]any{
						"name":              name,
						"status":            status,
						"bundled_version":   bundledVer,
						"installed_version": instVer,
					})
				}
				result["check"] = statuses
				result["install_dir"] = installDir
			}
			return a.ok(cmd, result)
		},
	}
	list.Flags().BoolVar(&listCheck, "check", false, "compare bundled vs installed skill versions")
	mark(list, risk.Read, "List available agent skills")

	read := &cobra.Command{
		Use:  "read NAME",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.format = firstNonEmpty(a.format, "json")
			root := skillsRoot()
			name := args[0]
			p := filepath.Join(root, name, "SKILL.md")
			b, err := os.ReadFile(p)
			if err != nil {
				return a.fail(cmd, risk.Read, fmt.Errorf("skill %q not found at %s: %w", name, p, err))
			}
			return a.ok(cmd, map[string]any{"name": name, "path": p, "content": string(b), "version": readSkillVersion(p)})
		},
	}
	mark(read, risk.Read, "Read a skill SKILL.md")

	pathCmd := &cobra.Command{
		Use:  "path [NAME]",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.format = firstNonEmpty(a.format, "json")
			root, source := skillsSourceMeta()
			if len(args) == 0 {
				return a.ok(cmd, map[string]any{"path": root, "source": source})
			}
			name := args[0]
			p := filepath.Join(root, name)
			if st, err := os.Stat(p); err != nil || !st.IsDir() {
				return a.fail(cmd, risk.Read, fmt.Errorf("skill %q not found under %s", name, root))
			}
			return a.ok(cmd, map[string]any{"name": name, "path": p, "skill_md": filepath.Join(p, "SKILL.md"), "source": source})
		},
	}
	mark(pathCmd, risk.Read, "Print skills directory or one skill path")

	cmd.AddCommand(list, read, pathCmd, a.newSkillsInstallCmd(), a.newSkillsUpdateCmd())
	return cmd
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
