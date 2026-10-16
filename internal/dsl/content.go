package dsl

import "strings"

// NormalizeContent prepares -q / --content for ES query_string.
// Standard analysis indexes dotted identifiers (e.g. java.lang.OutOfMemoryError)
// as one token; a bare fragment like OutOfMemoryError then misses.
//
// Wrapping rules (never wrap structured query_string expressions):
//   - already has * or ? -> leave as-is
//   - isStructuredQueryString -> leave as-is (even with forceWildcard)
//   - forceWildcard -> *value* (bare fragments like OutOfMemoryError)
//   - auto (default) + dotted identifier -> *value*
//   - disableAuto -> skip dotted auto-wrap
func NormalizeContent(content string, forceWildcard, disableAuto bool) string {
	v := strings.TrimSpace(content)
	if v == "" {
		return ""
	}
	if strings.ContainsAny(v, "*?") {
		return v
	}
	if isStructuredQueryString(v) {
		return v
	}
	if forceWildcard {
		return "*" + v + "*"
	}
	if !disableAuto && isDottedIdentifier(v) {
		return "*" + v + "*"
	}
	return v
}

// isStructuredQueryString reports Lucene/query_string expressions that must
// not be wrapped in *...* (operators, field:value, quotes, whitespace, etc.).
func isStructuredQueryString(s string) bool {
	upper := strings.ToUpper(s)
	for _, op := range []string{" AND ", " OR ", " NOT ", "&&", "||"} {
		if strings.Contains(upper, op) {
			return true
		}
	}
	if strings.ContainsAny(s, " \t\r\n\"':()[]{}\\/") {
		return true
	}
	return false
}

func isDottedIdentifier(s string) bool {
	if !strings.Contains(s, ".") {
		return false
	}
	return !isStructuredQueryString(s)
}
