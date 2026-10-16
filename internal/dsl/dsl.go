// Package dsl builds the Elasticsearch query body used by PlumeLog's Home.vue.
package dsl

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Search is the shortcut filter set from `query search` / `trace get`.
type Search struct {
	App        string
	ExcludeApp bool
	Env        string
	Level      string
	TraceID    string
	ClassName  string
	ServerName string
	Content    string
	Extend     []string
	FromMS     int64
	ToMS       int64
	Sort       string
	Highlight  bool
}

// Build returns the clientQuery/query JSON body.
func Build(s Search) ([]byte, error) {
	var must []any
	var mustNot []any

	if strings.TrimSpace(s.App) != "" {
		if s.ExcludeApp {
			for _, part := range strings.Split(s.App, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				mustNot = append(mustNot, matchPhrase("appName", part))
			}
			if len(mustNot) == 0 {
				return nil, fmt.Errorf("--exclude-app requires a non-empty --app")
			}
		} else {
			q := strings.TrimSpace(strings.ReplaceAll(s.App, ",", " "))
			if q != "" {
				must = append(must, matchPhrase("appName", q))
			}
		}
	} else if s.ExcludeApp {
		return nil, fmt.Errorf("--exclude-app requires --app")
	}

	if v := strings.TrimSpace(s.Env); v != "" {
		// Home.vue keeps commas inside env; other fields turn commas into spaces.
		must = append(must, matchPhrase("env", v))
	}
	if v := phrase(s.Level); v != "" {
		must = append(must, matchPhrase("logLevel", v))
	}
	if v := phrase(s.TraceID); v != "" {
		must = append(must, matchPhrase("traceId", v))
	}
	if v := phrase(s.ClassName); v != "" {
		must = append(must, matchPhrase("className", v))
	}
	if v := phrase(s.ServerName); v != "" {
		must = append(must, matchPhrase("serverName", v))
	}
	for _, raw := range s.Extend {
		field, value, err := splitExtend(raw)
		if err != nil {
			return nil, err
		}
		must = append(must, matchPhrase(field, value))
	}
	if v := strings.TrimSpace(s.Content); v != "" {
		must = append(must, map[string]any{
			"query_string": map[string]any{
				"query":         v,
				"default_field": "content",
			},
		})
	}
	if s.FromMS > 0 || s.ToMS > 0 {
		if s.FromMS > 0 && s.ToMS > 0 && s.FromMS > s.ToMS {
			return nil, fmt.Errorf("from-time is after to-time")
		}
		rng := map[string]any{}
		if s.FromMS > 0 {
			rng["gte"] = s.FromMS
		}
		if s.ToMS > 0 {
			rng["lt"] = s.ToMS
		}
		must = append(must, map[string]any{
			"range": map[string]any{"dtTime": rng},
		})
	}

	body := map[string]any{}
	switch {
	case len(must) == 0 && len(mustNot) == 0:
		body["query"] = map[string]any{"match_all": map[string]any{}}
	default:
		boolQ := map[string]any{}
		if len(must) > 0 {
			boolQ["must"] = must
		}
		if len(mustNot) > 0 {
			boolQ["must_not"] = mustNot
		}
		body["query"] = map[string]any{"bool": boolQ}
	}

	if s.Highlight {
		body["highlight"] = map[string]any{
			"fields": map[string]any{
				"content": map[string]any{"fragment_size": 2147483647},
			},
		}
	}

	sort, err := resolveSort(s)
	if err != nil {
		return nil, err
	}
	body["sort"] = sort
	return json.Marshal(body)
}

func resolveSort(s Search) (any, error) {
	if strings.TrimSpace(s.Sort) != "" {
		return parseSort(s.Sort)
	}
	if strings.TrimSpace(s.TraceID) != "" {
		return []any{
			map[string]string{"dtTime": "asc"},
			map[string]string{"seq": "asc"},
		}, nil
	}
	return []any{map[string]string{"dtTime": "desc"}}, nil
}

func parseSort(s string) (any, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, fmt.Errorf("sort JSON: %w", err)
		}
		return v, nil
	}
	var arr []any
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("sort %q: want field:asc|desc or a JSON array", s)
		}
		dir := strings.ToLower(strings.TrimSpace(kv[1]))
		if dir != "asc" && dir != "desc" {
			return nil, fmt.Errorf("sort direction %q must be asc or desc", dir)
		}
		field := strings.TrimSpace(kv[0])
		if field == "" {
			return nil, fmt.Errorf("sort field is empty")
		}
		arr = append(arr, map[string]string{field: dir})
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("sort is empty")
	}
	return arr, nil
}

func matchPhrase(field, query string) map[string]any {
	return map[string]any{
		"match_phrase": map[string]any{
			field: map[string]any{"query": query},
		},
	}
}

func phrase(raw string) string {
	return strings.TrimSpace(strings.ReplaceAll(raw, ",", " "))
}

func splitExtend(raw string) (string, string, error) {
	field, value, ok := strings.Cut(raw, "=")
	field = strings.TrimSpace(field)
	if !ok || field == "" {
		return "", "", fmt.Errorf("--extend %q: want field=value", raw)
	}
	return field, value, nil
}

// ParseWindow parses config durations such as 1h, 15m, or 1d.
func ParseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Hour, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("time window %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("time window %q", s)
	}
	return d, nil
}

// ParseTime accepts now, unix milliseconds, unix seconds, RFC3339, or
// "2006-01-02 15:04:05" / "2006-01-02" in loc.
func ParseTime(s string, now time.Time, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "now" {
		return now, nil
	}
	if loc == nil {
		loc = time.Local
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		switch {
		case n >= 1_000_000_000_000:
			return time.UnixMilli(n), nil
		case n >= 1_000_000_000:
			return time.Unix(n, 0), nil
		default:
			return time.Time{}, fmt.Errorf("time %q is not unix seconds or milliseconds", s)
		}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q (want ms, RFC3339, or YYYY-MM-DD HH:MM:SS)", s)
}

// ResolveWindow applies the default window and clamps a future to-time to now.
func ResolveWindow(fromRaw, toRaw string, window time.Duration, now time.Time) (fromMS, toMS int64, err error) {
	if window <= 0 {
		window = time.Hour
	}
	to := now
	if strings.TrimSpace(toRaw) != "" {
		to, err = ParseTime(toRaw, now, now.Location())
		if err != nil {
			return 0, 0, err
		}
	}
	if to.After(now) {
		to = now
	}
	from := to.Add(-window)
	if strings.TrimSpace(fromRaw) != "" {
		from, err = ParseTime(fromRaw, now, now.Location())
		if err != nil {
			return 0, 0, err
		}
	}
	if from.After(to) {
		return 0, 0, fmt.Errorf("from-time is after to-time")
	}
	return from.UnixMilli(), to.UnixMilli(), nil
}

// AppNamesAgg is the Home.vue queryAppNames body.
func AppNamesAgg() []byte {
	return []byte(`{"size":0,"aggregations":{"dataCount":{"terms":{"size":1000,"field":"appNameWithEnv"}}}}`)
}
