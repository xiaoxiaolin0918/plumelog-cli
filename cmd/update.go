package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xiaoxiaolin0918/plumelog-cli/internal/output"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/risk"
	"github.com/xiaoxiaolin0918/plumelog-cli/internal/version"
)

const defaultUpdateAPIURL = "https://api.github.com/repos/xiaoxiaolin0918/plumelog-cli/releases/latest"

// Test hooks (override in unit tests).
var (
	updateHTTPClient = &http.Client{Timeout: 90 * time.Second}
	updateAPIURL     = defaultUpdateAPIURL
	updateExecutable = os.Executable
	updateGOOS       = runtime.GOOS
	updateGOARCH     = runtime.GOARCH
)

type ghReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type ghRelease struct {
	TagName string           `json:"tag_name"`
	HTMLURL string           `json:"html_url"`
	Assets  []ghReleaseAsset `json:"assets"`
}

type updateCheckResult struct {
	Name        string `json:"name"`
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	Status      string `json:"status"` // up_to_date | outdated | ahead | unknown
	ReleaseURL  string `json:"release_url,omitempty"`
	Asset       string `json:"asset,omitempty"`
	AssetURL    string `json:"asset_url,omitempty"`
	ChecksumURL string `json:"checksum_url,omitempty"`
	Platform    string `json:"platform"`
	Hint        string `json:"hint,omitempty"`
}

type updateApplyResult struct {
	updateCheckResult
	Applied  bool   `json:"applied"`
	Binary   string `json:"binary,omitempty"`
	Backup   string `json:"backup,omitempty"`
	Verified bool   `json:"checksum_ok"`
}

func (a *App) newUpdateCmd() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use: "update",
		Long: `Check GitHub Releases for a newer plumelog binary, or install one.

Uses the latest release of xiaoxiaolin0918/plumelog-cli (assets from release.yml:
plumelog_<os>_<arch>[.exe] + checksums.txt).

  plumelog update           # same as --check
  plumelog update --check   # compare current vs latest; do not download
  plumelog update --yes     # download matching asset, verify SHA-256, replace this binary

mgr invokes: <bin> update [--check|--yes].`,
		Example: `  plumelog update --check
  plumelog update --yes
  plumelog update --yes --dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a.format = firstNonEmpty(a.format, "json")
			if checkOnly || !a.yes {
				res, err := a.checkUpdate(cmd)
				if err != nil {
					return a.fail(cmd, risk.Read, err)
				}
				return a.ok(cmd, res)
			}
			// --yes: high-risk self-replace
			policy := risk.HighRisk(false, false)
			if a.dryRun {
				res, err := a.checkUpdate(cmd)
				if err != nil {
					return a.fail(cmd, policy.Level, err)
				}
				env := output.Envelope{
					OK:           true,
					DryRun:       true,
					Risk:         policy.Level,
					WouldRequire: []string{},
					Result: output.MustResult(map[string]any{
						"would":   "download+verify+replace",
						"current": res.Current,
						"latest":  res.Latest,
						"status":  res.Status,
						"asset":   res.Asset,
						"asset_url": res.AssetURL,
						"binary":  mustExePath(),
					}),
				}
				return a.write(cmd, env)
			}
			if err := policy.Check(a.yes, "", commandAction(cmd)); err != nil {
				if g, ok := err.(risk.GateResult); ok {
					return a.failConfirmation(cmd, g)
				}
				return a.fail(cmd, policy.Level, err)
			}
			res, err := a.applyUpdate(cmd)
			if err != nil {
				return a.fail(cmd, policy.Level, err)
			}
			return a.ok(cmd, res)
		},
	}
	mark(cmd, risk.High, "Self-update from GitHub Releases (use --check to only compare)")
	// Override Short/risk annotation nuance: --check is read; command defaults high because install needs --yes.
	// Keep annotation high-risk-write so agents always pass --yes for apply; --check path never mutates.
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only check latest release; do not download or replace")
	return cmd
}

func mustExePath() string {
	p, err := updateExecutable()
	if err != nil || p == "" {
		return ""
	}
	return p
}

func releaseAssetName(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		if goarch == "amd64" {
			return "plumelog_windows_amd64.exe", nil
		}
	case "linux":
		switch goarch {
		case "amd64":
			return "plumelog_linux_amd64", nil
		case "arm64":
			return "plumelog_linux_arm64", nil
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "plumelog_darwin_amd64", nil
		case "arm64":
			return "plumelog_darwin_arm64", nil
		}
	}
	return "", fmt.Errorf("unsupported platform %s/%s (no release asset)", goos, goarch)
}

func statusVsLatest(current, latestTag string) string {
	latest := strings.TrimSpace(latestTag)
	rel := CompareSemver(current, latest)
	switch rel {
	case RelEqual:
		return "up_to_date"
	case RelOutdated:
		return "outdated"
	case RelAhead:
		return "ahead"
	default:
		return "unknown"
	}
}

func parseChecksums(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// sha256sum: "<hex>  name" or "<hex> *name"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		name := fields[1]
		name = strings.TrimPrefix(name, "*")
		name = filepath.Base(name)
		if len(sum) != 64 {
			continue
		}
		out[name] = sum
	}
	return out
}

func (a *App) fetchLatestRelease(cmd *cobra.Command) (*ghRelease, error) {
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, updateAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", version.UserAgent)
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub Releases HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, fmt.Errorf("release missing tag_name")
	}
	return &rel, nil
}

func findAsset(rel *ghRelease, name string) (ghReleaseAsset, bool) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return ghReleaseAsset{}, false
}

func (a *App) checkUpdate(cmd *cobra.Command) (*updateCheckResult, error) {
	rel, err := a.fetchLatestRelease(cmd)
	if err != nil {
		return nil, err
	}
	assetName, err := releaseAssetName(updateGOOS, updateGOARCH)
	if err != nil {
		return nil, err
	}
	asset, ok := findAsset(rel, assetName)
	sumAsset, _ := findAsset(rel, "checksums.txt")
	res := &updateCheckResult{
		Name:        version.Name,
		Current:     version.Version,
		Latest:      strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v"),
		Status:      statusVsLatest(version.Version, rel.TagName),
		ReleaseURL:  rel.HTMLURL,
		Asset:       assetName,
		Platform:    updateGOOS + "/" + updateGOARCH,
		ChecksumURL: sumAsset.BrowserDownloadURL,
	}
	if ok {
		res.AssetURL = asset.BrowserDownloadURL
	} else {
		res.Hint = "no matching asset in latest release; download manually from release_url"
	}
	switch res.Status {
	case "outdated":
		res.Hint = firstNonEmpty(res.Hint, "run: plumelog update --yes")
	case "up_to_date":
		res.Hint = firstNonEmpty(res.Hint, "already on latest release")
	case "ahead":
		res.Hint = firstNonEmpty(res.Hint, "local version is newer than latest release tag")
	}
	return res, nil
}

func (a *App) downloadURL(cmd *cobra.Command, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.UserAgent)
	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

func (a *App) applyUpdate(cmd *cobra.Command) (*updateApplyResult, error) {
	check, err := a.checkUpdate(cmd)
	if err != nil {
		return nil, err
	}
	out := &updateApplyResult{updateCheckResult: *check}
	if check.Status == "up_to_date" {
		out.Applied = false
		out.Hint = "already on latest release; nothing to install"
		out.Binary = mustExePath()
		return out, nil
	}
	if check.AssetURL == "" {
		return nil, fmt.Errorf("no asset URL for %s", check.Asset)
	}
	if check.ChecksumURL == "" {
		return nil, fmt.Errorf("checksums.txt missing from release")
	}

	sumBody, err := a.downloadURL(cmd, check.ChecksumURL)
	if err != nil {
		return nil, fmt.Errorf("download checksums: %w", err)
	}
	sums := parseChecksums(string(sumBody))
	want, ok := sums[check.Asset]
	if !ok {
		return nil, fmt.Errorf("checksums.txt has no entry for %s", check.Asset)
	}

	binBody, err := a.downloadURL(cmd, check.AssetURL)
	if err != nil {
		return nil, fmt.Errorf("download binary: %w", err)
	}
	sum := sha256.Sum256(binBody)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return nil, fmt.Errorf("checksum mismatch for %s: got %s want %s", check.Asset, got, want)
	}
	out.Verified = true

	exe, err := updateExecutable()
	if err != nil || exe == "" {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return nil, err
	}
	out.Binary = exe

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, "plumelog-update-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("temp file beside binary: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(binBody); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Chmod(0o755); err != nil {
		// Windows may ignore; continue
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	bak := exe + ".bak"
	_ = os.Remove(bak)
	if err := os.Rename(exe, bak); err != nil {
		return nil, fmt.Errorf("backup current binary: %w (close other plumelog processes and retry)", err)
	}
	out.Backup = bak
	if err := os.Rename(tmpName, exe); err != nil {
		_ = os.Rename(bak, exe) // best-effort rollback
		return nil, fmt.Errorf("replace binary: %w", err)
	}
	// Drop backup on success (Windows may still lock; ignore).
	_ = os.Remove(bak)
	out.Backup = ""
	out.Applied = true
	out.Status = "up_to_date"
	out.Current = check.Latest
	out.Hint = "binary replaced; re-run plumelog version to confirm"
	return out, nil
}
