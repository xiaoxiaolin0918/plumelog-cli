// Package output prints the JSON ok-envelope used by every command.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/itchyny/gojq"
)

// Envelope is the agent-facing JSON document.
// Server payloads stay under result. Dry-run fields sit beside ok.
type Envelope struct {
	OK           bool              `json:"ok"`
	DryRun       bool              `json:"dry_run,omitempty"`
	Risk         string            `json:"risk,omitempty"`
	Error        string            `json:"error,omitempty"`
	Hint         string            `json:"hint,omitempty"`
	Method       string            `json:"method,omitempty"`
	URL          string            `json:"url,omitempty"`
	FallbackURLs []string          `json:"fallback_urls,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         json.RawMessage   `json:"body,omitempty"`
	BodyText     string            `json:"body_text,omitempty"`
	CookieNames  []string          `json:"cookie_names,omitempty"`
	WouldRequire []string          `json:"would_require,omitempty"`
	Result       json.RawMessage   `json:"result,omitempty"`
}

// Write formats env as json, pretty, or a best-effort table, then scrubs secrets.
func Write(w io.Writer, format, jqExpr string, env Envelope, secrets []string) error {
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if strings.TrimSpace(jqExpr) != "" {
		raw, err = applyJQ(raw, jqExpr, format)
		if err != nil {
			return err
		}
	} else {
		raw, err = formatRaw(raw, format)
		if err != nil {
			return err
		}
	}
	raw = Scrub(raw, secrets)
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		raw = append(raw, '\n')
	}
	_, err = w.Write(raw)
	return err
}

func formatRaw(raw []byte, format string) ([]byte, error) {
	switch format {
	case "", "json":
		return raw, nil
	case "pretty":
		var buf bytes.Buffer
		if err := json.Indent(&buf, raw, "", "  "); err != nil {
			return raw, nil
		}
		return buf.Bytes(), nil
	case "table":
		if t, ok := renderTable(raw); ok {
			return t, nil
		}
		var buf bytes.Buffer
		if err := json.Indent(&buf, raw, "", "  "); err != nil {
			return raw, nil
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown --format %q (want json, pretty, or table)", format)
	}
}

func applyJQ(raw []byte, expr, format string) ([]byte, error) {
	var input any
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("jq: %w", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("jq: %w", err)
	}
	var values []any
	iter := code.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if halt, ok := err.(*gojq.HaltError); ok && halt.Value() == nil {
				break
			}
			return nil, fmt.Errorf("jq: %w", err)
		}
		values = append(values, v)
	}
	var rendered [][]byte
	for _, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		b, err = formatRaw(b, format)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, bytes.TrimSpace(b))
	}
	if len(rendered) == 0 {
		return []byte("null"), nil
	}
	return bytes.Join(rendered, []byte("\n")), nil
}

// Scrub replaces secret substrings of length >= 4 with ***.
func Scrub(b []byte, secrets []string) []byte {
	s := string(b)
	seen := map[string]struct{}{}
	for _, sec := range secrets {
		if len(sec) < 4 {
			continue
		}
		if _, ok := seen[sec]; ok {
			continue
		}
		seen[sec] = struct{}{}
		s = strings.ReplaceAll(s, sec, "***")
	}
	return []byte(s)
}

func renderTable(raw []byte) ([]byte, bool) {
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, false
	}
	result, ok := env["result"]
	if !ok {
		return nil, false
	}
	rows, ok := asRows(result)
	if !ok || len(rows) == 0 {
		return nil, false
	}
	cols := map[string]struct{}{}
	for _, row := range rows {
		for k := range row {
			cols[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(cols))
	for k := range cols {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteString(strings.Join(keys, "\t"))
	buf.WriteByte('\n')
	for _, row := range rows {
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte('\t')
			}
			buf.WriteString(cell(row[k]))
		}
		buf.WriteByte('\n')
	}
	return buf.Bytes(), true
}

func asRows(v any) ([]map[string]any, bool) {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return nil, false
	}
	rows := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		rows = append(rows, m)
	}
	return rows, true
}

func cell(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.ReplaceAll(t, "\t", " ")
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// RawResult wraps a JSON value or a plain string for Envelope.Result.
func RawResult(body []byte) json.RawMessage {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil
	}
	if json.Valid(body) {
		return json.RawMessage(body)
	}
	b, err := json.Marshal(string(body))
	if err != nil {
		return nil
	}
	return b
}

// MustResult marshals v into Envelope.Result.
func MustResult(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}
