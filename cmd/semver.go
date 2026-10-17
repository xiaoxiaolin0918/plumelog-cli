package cmd

import (
	"strconv"
	"strings"
)

// VersionRelation is installed relative to bundled for skills list --check.
type VersionRelation string

const (
	RelEqual    VersionRelation = "equal"
	RelOutdated VersionRelation = "outdated" // installed < bundled
	RelAhead    VersionRelation = "ahead"    // installed > bundled
	RelInvalid  VersionRelation = "invalid"  // not comparable as semver
)

// CompareSemver compares installed to bundled (direction: installed relative to bundled).
// Accepts optional leading "v"/"V" and numeric major.minor.patch (extra numeric segment ignored).
// Pre-release/build (+/-) suffixes make the version invalid for ordering.
func CompareSemver(installed, bundled string) VersionRelation {
	inst, okI := parseSemverCore(installed)
	bund, okB := parseSemverCore(bundled)
	if !okI || !okB {
		return RelInvalid
	}
	for i := 0; i < 3; i++ {
		if inst[i] < bund[i] {
			return RelOutdated
		}
		if inst[i] > bund[i] {
			return RelAhead
		}
	}
	return RelEqual
}

func parseSemverCore(s string) ([3]int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return [3]int{}, false
	}
	if s[0] == 'v' || s[0] == 'V' {
		s = s[1:]
	}
	if strings.ContainsAny(s, "+-") {
		return [3]int{}, false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 4 {
		return [3]int{}, false
	}
	var out [3]int
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			break
		}
		p := parts[i]
		if p == "" {
			return [3]int{}, false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return [3]int{}, false
			}
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	if len(parts) == 4 {
		p := parts[3]
		if p == "" {
			return [3]int{}, false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return [3]int{}, false
			}
		}
	}
	return out, true
}
