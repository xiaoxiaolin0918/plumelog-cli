package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/config"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/version"
)

const updateCheckCacheTTL = 24 * time.Hour

// Short client for opportunistic background checks (never blocks business long).
var updateCheckHTTPClient = &http.Client{Timeout: 3 * time.Second}

type updateCheckCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
	Status    string    `json:"status"`
	Notified  string    `json:"notified_version,omitempty"`
}

func (a *App) resolveUpdateFlags() {
	if a.flagCheckUpdate {
		a.checkUpdateEnabled = true
	}
	if a.flagAutoUpdate {
		a.autoUpdateEnabled = true
	}
	if v, explicit := envBool("PLUMELOG_CHECK_UPDATE"); explicit && v {
		a.checkUpdateEnabled = true
	}
	if v, explicit := envBool("PLUMELOG_AUTO_UPDATE"); explicit && v {
		a.autoUpdateEnabled = true
	}
	if a.cfg.CheckUpdate {
		a.checkUpdateEnabled = true
	}
	if a.cfg.AutoUpdate {
		a.autoUpdateEnabled = true
	}
	if a.autoUpdateEnabled {
		a.checkUpdateEnabled = true
	}
	a.updateFlagsResolved = true
}

func (a *App) ensureUpdateFlags() {
	if a.updateFlagsResolved {
		return
	}
	// Best-effort config load for commands that skip resolveInto (e.g. version).
	if !a.cfg.CheckUpdate && !a.cfg.AutoUpdate {
		if dir, err := config.Dir(); err == nil {
			name, err := config.ResolveProfileName(a.flagProfile, os.Getenv("PLUMELOG_PROFILE"), dir)
			if err == nil {
				path, _, err := config.PathsForProfile(dir, name)
				if err == nil {
					if file, _, err := config.Load(path); err == nil {
						a.cfg = file
					}
				}
			}
		}
	}
	a.resolveUpdateFlags()
}

func updateCheckCachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update-check-cache.json"), nil
}

func loadUpdateCheckCache() (updateCheckCache, bool) {
	path, err := updateCheckCachePath()
	if err != nil {
		return updateCheckCache{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return updateCheckCache{}, false
	}
	var c updateCheckCache
	if json.Unmarshal(b, &c) != nil {
		return updateCheckCache{}, false
	}
	return c, true
}

func saveUpdateCheckCache(c updateCheckCache) {
	path, err := updateCheckCachePath()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o600)
}

func shouldSkipBackgroundUpdate(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}
	switch cmd.Name() {
	case "help", "completion", "update":
		return true
	}
	if isSkillsCommand(cmd) {
		return true
	}
	return false
}

// maybeBackgroundUpdate runs after a successful command when check-update is enabled.
// Never fails the main command. Hints go to stderr only (never JSON stdout).
func (a *App) maybeBackgroundUpdate(cmd *cobra.Command) {
	if a == nil {
		return
	}
	a.ensureUpdateFlags()
	if !a.checkUpdateEnabled {
		return
	}
	if shouldSkipBackgroundUpdate(cmd) {
		return
	}
	cache, ok := loadUpdateCheckCache()
	if ok && time.Since(cache.CheckedAt) < updateCheckCacheTTL && cache.Latest != "" {
		if cache.Status == "outdated" && CompareSemver(version.Version, cache.Latest) == RelOutdated {
			a.emitUpdateHint(cmd, cache.Latest, false)
		}
		return
	}
	res, err := a.quickCheckLatest(cmd.Context())
	if err != nil || res == nil {
		return
	}
	cache = updateCheckCache{
		CheckedAt: time.Now().UTC(),
		Latest:    res.Latest,
		Status:    res.Status,
	}
	saveUpdateCheckCache(cache)
	if res.Status != "outdated" {
		return
	}
	applied := false
	if a.autoUpdateEnabled && !a.dryRun {
		applied = a.trySilentAutoUpdate(cmd)
	}
	a.emitUpdateHint(cmd, res.Latest, applied)
}

func (a *App) emitUpdateHint(cmd *cobra.Command, latest string, applied bool) {
	w := cmd.ErrOrStderr()
	if applied {
		fmt.Fprintf(w, "plumelog: auto-updated to %s (was %s); re-run if needed\n", latest, version.Version)
		return
	}
	fmt.Fprintf(w, "plumelog: newer version %s available (current %s); run: plumelog update --yes\n", latest, version.Version)
}

func (a *App) quickCheckLatest(ctx context.Context) (*updateCheckResult, error) {
	old := updateHTTPClient
	updateHTTPClient = updateCheckHTTPClient
	defer func() { updateHTTPClient = old }()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	return a.checkUpdate(cmd)
}

func (a *App) trySilentAutoUpdate(cmd *cobra.Command) bool {
	res, err := a.applyUpdate(cmd)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "plumelog: auto-update failed: %v (rolled back if possible)\n", err)
		return false
	}
	return res != nil && res.Applied
}

// attachBackgroundUpdateHook wires PersistentPostRunE once on root.
func attachBackgroundUpdateHook(root *cobra.Command, a *App) {
	prev := root.PersistentPostRunE
	root.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if prev != nil {
			if err := prev(cmd, args); err != nil {
				return err
			}
		}
		a.maybeBackgroundUpdate(cmd)
		return nil
	}
}
