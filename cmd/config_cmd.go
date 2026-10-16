package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"plumelog-cli/internal/config"
	"plumelog-cli/internal/dsl"
	"plumelog-cli/internal/risk"
)

func (a *App) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and write config.json (passwords are refused)",
		RunE:  helpOnly,
	}
	cmd.AddCommand(a.newConfigGet(), a.newConfigSet(), a.newConfigPath(), a.newConfigShow())
	return cmd
}

func (a *App) newConfigPath() *cobra.Command {
	cmd := &cobra.Command{
		Use: "path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.ok(cmd, map[string]any{
				"config_path": a.configPath,
				"cookie_path": a.cookiePath,
				"config_dir":  dirOf(a.configPath),
			})
		},
	}
	mark(cmd, risk.Read, "Print config and cookie jar paths")
	return cmd
}

func (a *App) newConfigShow() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "show",
		Long: "Print the stored config and the effective values. Passwords and cookie values are never included.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.ok(cmd, map[string]any{
				"config_path": a.configPath,
				"cookie_path": a.cookiePath,
				"stored":      storedView(a.cfg),
				"effective": map[string]any{
					"base_url":            a.baseURL,
					"path_prefix":         a.prefixMode,
					"format":              a.format,
					"timeout":             a.timeout.String(),
					"default_size":        a.defaultSize,
					"default_time_window": a.timeWindow.String(),
					"login_username":      a.cfg.LoginUsername,
				},
				"env": map[string]string{
					"PLUMELOG_ADMIN_PASSWORD": presence(os.Getenv("PLUMELOG_ADMIN_PASSWORD")),
					"PLUMELOG_LOGIN_PASSWORD": presence(os.Getenv("PLUMELOG_LOGIN_PASSWORD")),
					"PLUMELOG_BASE_URL":       presence(os.Getenv("PLUMELOG_BASE_URL")),
					"PLUMELOG_PATH_PREFIX":    presence(os.Getenv("PLUMELOG_PATH_PREFIX")),
					"PLUMELOG_LOGIN_USERNAME": presence(os.Getenv("PLUMELOG_LOGIN_USERNAME")),
				},
			})
		},
	}
	mark(cmd, risk.Read, "Show config with secrets redacted")
	return cmd
}

func (a *App) newConfigGet() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "get <key>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if config.SecretKey(key) {
				return a.fail(cmd, risk.Read, fmt.Errorf("refusing to print %s", key))
			}
			val, ok, err := storedValue(a.cfg, key)
			if err != nil {
				return a.fail(cmd, risk.Read, err)
			}
			return a.ok(cmd, map[string]any{"key": key, "set": ok, "value": val})
		},
	}
	mark(cmd, risk.Read, "Print one stored config value")
	return cmd
}

func (a *App) newConfigSet() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "set <key> <value>",
		Args: cobra.ExactArgs(2),
		Long: "Write one field to config.json (mode 0600).\nPasswords are refused. path_prefix accepts auto, root, or /plumelog.",
		Example: `  plumelog config set base_url http://127.0.0.1:8891
  plumelog config set path_prefix /plumelog
  plumelog config set path_prefix root
  plumelog config set default_time_window 1h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, val := args[0], args[1]
			if config.SecretKey(key) {
				return a.fail(cmd, risk.Write, fmt.Errorf("refusing to store %s in config.json; use a flag or environment variable", key))
			}
			if err := applySet(&a.cfg, key, val); err != nil {
				return a.fail(cmd, risk.Write, err)
			}
			if err := config.Save(a.configPath, a.cfg); err != nil {
				return a.fail(cmd, risk.Write, err)
			}
			return a.ok(cmd, map[string]any{"key": key, "config_path": a.configPath})
		},
	}
	mark(cmd, risk.Write, "Set one non-secret config field")
	return cmd
}

func presence(v string) string {
	if v == "" {
		return "unset"
	}
	return "set"
}

func dirOf(path string) string {
	i := strings.LastIndex(path, string(os.PathSeparator))
	if i < 0 {
		return path
	}
	return path[:i]
}

func storedView(f config.File) map[string]any {
	prefix := any("auto")
	if f.PathPrefix != nil {
		if *f.PathPrefix == "" {
			prefix = "root"
		} else {
			prefix = *f.PathPrefix
		}
	}
	return map[string]any{
		"base_url":            f.BaseURL,
		"path_prefix":         prefix,
		"timeout_sec":         f.TimeoutSec,
		"default_size":        f.DefaultSize,
		"default_time_window": f.DefaultTimeWindow,
		"format":              f.Format,
		"cookie_jar":          f.CookieJar,
		"login_username":      f.LoginUsername,
	}
}

func storedValue(f config.File, key string) (any, bool, error) {
	switch key {
	case "base_url":
		return f.BaseURL, f.BaseURL != "", nil
	case "path_prefix":
		if f.PathPrefix == nil {
			return "auto", false, nil
		}
		if *f.PathPrefix == "" {
			return "root", true, nil
		}
		return *f.PathPrefix, true, nil
	case "timeout_sec":
		return f.TimeoutSec, f.TimeoutSec > 0, nil
	case "default_size":
		return f.DefaultSize, f.DefaultSize > 0, nil
	case "default_time_window":
		return f.DefaultTimeWindow, f.DefaultTimeWindow != "", nil
	case "format":
		return f.Format, f.Format != "", nil
	case "cookie_jar":
		return f.CookieJar, f.CookieJar != "", nil
	case "login_username":
		return f.LoginUsername, f.LoginUsername != "", nil
	default:
		return nil, false, fmt.Errorf("unknown key %q (want %s)", key, strings.Join(config.Keys(), ", "))
	}
}

func applySet(f *config.File, key, val string) error {
	switch key {
	case "base_url":
		val = strings.TrimRight(strings.TrimSpace(val), "/")
		if !strings.Contains(val, "://") {
			return fmt.Errorf("base_url must look like http://localhost:8891")
		}
		f.BaseURL = val
	case "path_prefix":
		switch strings.TrimSpace(val) {
		case "auto":
			f.PathPrefix = nil
		case "root", `""`:
			s := ""
			f.PathPrefix = &s
		case "/plumelog":
			s := "/plumelog"
			f.PathPrefix = &s
		default:
			return fmt.Errorf("path_prefix must be auto, root, or /plumelog")
		}
	case "timeout_sec":
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("timeout_sec must be a positive integer")
		}
		f.TimeoutSec = n
	case "default_size":
		n, err := strconv.Atoi(val)
		if err != nil || n <= 0 {
			return fmt.Errorf("default_size must be a positive integer")
		}
		f.DefaultSize = n
	case "default_time_window":
		if _, err := dsl.ParseWindow(val); err != nil {
			return err
		}
		f.DefaultTimeWindow = val
	case "format":
		switch val {
		case "json", "pretty", "table":
			f.Format = val
		default:
			return fmt.Errorf("format must be json, pretty, or table")
		}
	case "cookie_jar":
		if strings.TrimSpace(val) == "" {
			return fmt.Errorf("cookie_jar path is empty")
		}
		f.CookieJar = val
	case "login_username":
		f.LoginUsername = val
	default:
		return fmt.Errorf("unknown key %q (want %s)", key, strings.Join(config.Keys(), ", "))
	}
	return nil
}
