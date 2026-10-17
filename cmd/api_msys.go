package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MSYS path mangling: git-bash / MSYS2 / Cygwin rewrite argv that starts with
// "/" into Windows paths BEFORE the child is exec'd, so
//
//	plumelog api POST /clientQuery
//
// arrives as "C:/Program Files/Git/clientQuery". The CLI then hits a broken
// URL. unmangleAPIPathArg restores the original /path when confident.
//
// Detection is strict (better leave untouched than invent a path):
//   - "//path" double-slash escape collapses to "/path" when it looks like an API path
//   - "<drive>:/<msys-root>/..." restores when the root probes as an MSYS install,
//     or when an MSYS env signal is present and a known API marker is found
//
// PLUMELOG_API_NO_UNMANGLE=1 disables all rewriting.

const envAPIUnmangleOff = "PLUMELOG_API_NO_UNMANGLE"

// msysNoteOut receives the one-line stderr note when a path was restored.
var msysNoteOut io.Writer = os.Stderr

// knownAPIPathMarkers are substrings that identify a PlumeLog API path inside a
// mangled Windows argument (aligned with risk.ForAPI + /plumelog/ alias).
var knownAPIPathMarkers = []string{
	"/plumelog/",
	"/clientQuery",
	"/getRunModel",
	"/login",
	"/logout",
	"/deleteQueue",
	"/deleteIndex",
	"/resetIndex",
	"/deleteByQuery",
	"/deleteWarningRule",
	"/delExtendfield",
	"/getAppNames",
	"/queryAppName",
	"/queryAppNames",
	"/getWarningRuleList",
	"/getQueueCounts",
	"/getServerInfo",
	"/getExtendFieldList",
	"/saveWarningRuleList",
	"/addExtendfield",
	"/sendLog",
	"/sendRunLog",
	"/sendTraceLog",
	"/query",
}

// unmangleAPIPathArg normalizes the <path> argument of `plumelog api`.
// Returns the (possibly unchanged) path and a stderr note ("" when unchanged).
func unmangleAPIPathArg(arg string) (string, string) {
	if strings.TrimSpace(arg) == "" || apiUnmangleDisabled() {
		return arg, ""
	}
	// git-bash escape: "//clientQuery" reaches argv with the extra leading slash.
	if strings.HasPrefix(arg, "//") {
		candidate := arg[1:]
		if looksLikeAPIPath(candidate) {
			return candidate, msysUnmangleNote(arg, candidate)
		}
	}
	restored, ok := restoreMSYSMangledAPIPath(arg, msysEnvSignal(), probeMSYSRootOnDisk)
	if !ok {
		return arg, ""
	}
	return restored, msysUnmangleNote(arg, restored)
}

// restoreMSYSMangledAPIPath restores "<drive>:/<msys-root>/<api-path>" to "/<api-path>".
func restoreMSYSMangledAPIPath(arg string, envSignal bool, probeRoot func(string) bool) (string, bool) {
	if probeRoot == nil {
		probeRoot = func(string) bool { return false }
	}
	if !isWindowsDrivePath(arg) {
		return "", false
	}
	// Prefer a disk-proven MSYS root (walk every directory boundary).
	for i := 3; i < len(arg); i++ {
		if arg[i] != '/' && arg[i] != '\\' {
			continue
		}
		root := arg[:i]
		if !probeRoot(root) {
			continue
		}
		rest := filepath.ToSlash(arg[i:])
		if looksLikeAPIPath(rest) {
			return rest, true
		}
	}
	// No proven root: only restore with an MSYS env signal + known API marker.
	if !envSignal {
		return "", false
	}
	bestIdx := -1
	for _, marker := range knownAPIPathMarkers {
		for _, idx := range allIndexes(arg, marker) {
			if idx <= 0 || !isWindowsDrivePath(arg[:idx]) {
				continue
			}
			if !markerBoundaryOK(arg, idx, marker) {
				continue
			}
			if bestIdx < 0 || idx < bestIdx {
				bestIdx = idx
			}
		}
	}
	if bestIdx > 0 {
		return filepath.ToSlash(arg[bestIdx:]), true
	}
	return "", false
}

// markerBoundaryOK rejects accidental prefixes (e.g. /query inside /queryAppName).
func markerBoundaryOK(arg string, idx int, marker string) bool {
	end := idx + len(marker)
	if end > len(arg) {
		return false
	}
	if end == len(arg) {
		return true
	}
	c := arg[end]
	if c == '/' || c == '?' {
		return true
	}
	// Markers that already end with "/" own the next segment.
	return strings.HasSuffix(marker, "/")
}

func msysUnmangleNote(from, to string) string {
	return fmt.Sprintf(
		"note: restored MSYS-mangled api path %q -> %q (git-bash/MSYS rewrites leading-/ args to Windows paths); "+
			"alternatives: MSYS_NO_PATHCONV=1, MSYS2_ARG_CONV_EXCL='*', or a double slash %q; disable with %s=1",
		from, to, "//clientQuery", envAPIUnmangleOff)
}

func apiUnmangleDisabled() bool {
	v := strings.TrimSpace(os.Getenv(envAPIUnmangleOff))
	return v != "" && v != "0"
}

func msysEnvSignal() bool {
	return os.Getenv("MSYSTEM") != "" || os.Getenv("MSYS") != ""
}

func probeMSYSRootOnDisk(root string) bool {
	if root == "" {
		return false
	}
	if fi, err := os.Stat(filepath.Join(root, "usr", "bin")); err == nil && fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(filepath.Join(root, "bin", "bash.exe")); err == nil && !fi.IsDir() {
		return true
	}
	return false
}

func isWindowsDrivePath(p string) bool {
	if len(p) < 3 {
		return false
	}
	drive := p[0]
	if !((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) {
		return false
	}
	return p[1] == ':' && (p[2] == '/' || p[2] == '\\')
}

func looksLikeAPIPath(p string) bool {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return false
	}
	base := p
	if i := strings.IndexByte(base, '?'); i >= 0 {
		base = base[:i]
	}
	// Reject leaked drive segments like /C:/...
	if len(base) >= 4 && base[0] == '/' && base[2] == ':' && ((base[1] >= 'A' && base[1] <= 'Z') || (base[1] >= 'a' && base[1] <= 'z')) {
		return false
	}
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".dll") || strings.HasSuffix(lower, ".bat") {
		return false
	}
	return true
}

func allIndexes(s, sub string) []int {
	var out []int
	for i := 0; i+len(sub) <= len(s); {
		j := strings.Index(s[i:], sub)
		if j < 0 {
			break
		}
		out = append(out, i+j)
		i += j + 1
	}
	return out
}
