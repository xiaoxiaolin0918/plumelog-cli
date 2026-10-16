package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/risk"
)

// skillsRoot finds the bundled skills/ directory (next to binary, source tree, or cwd).
func skillsRoot() string {
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
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return "skills"
}

func isSkillsCommand(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == "skills" {
			return true
		}
	}
	return false
}

func (a *App) newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "List, read, or install bundled agent skill content",
		RunE:  helpOnly,
	}

	list := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.format = firstNonEmpty(a.format, "json")
			root := skillsRoot()
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
			return a.ok(cmd, map[string]any{"skills": names, "skills_dir": root})
		},
	}
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
			return a.ok(cmd, map[string]any{"name": name, "path": p, "content": string(b)})
		},
	}
	mark(read, risk.Read, "Read a skill SKILL.md")

	pathCmd := &cobra.Command{
		Use:  "path [NAME]",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a.format = firstNonEmpty(a.format, "json")
			root := skillsRoot()
			if len(args) == 0 {
				return a.ok(cmd, map[string]any{"path": root})
			}
			name := args[0]
			p := filepath.Join(root, name)
			if st, err := os.Stat(p); err != nil || !st.IsDir() {
				return a.fail(cmd, risk.Read, fmt.Errorf("skill %q not found under %s", name, root))
			}
			return a.ok(cmd, map[string]any{"name": name, "path": p, "skill_md": filepath.Join(p, "SKILL.md")})
		},
	}
	mark(pathCmd, risk.Read, "Print skills directory or one skill path")

	cmd.AddCommand(list, read, pathCmd, a.newSkillsInstallCmd())
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