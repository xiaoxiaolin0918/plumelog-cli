package dsl

import (
	"encoding/json"
	"fmt"
)

// EnsureDtTimeRange injects or replaces a dtTime range clause in an Elasticsearch
// query body so raw DSL used with --index auto-align matches query search.
// Existing range.dtTime clauses under bool.must / bool.filter are removed first.
func EnsureDtTimeRange(body []byte, fromMS, toMS int64) ([]byte, error) {
	if fromMS <= 0 && toMS <= 0 {
		return body, nil
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("DSL JSON: %w", err)
	}
	if root == nil {
		root = map[string]any{}
	}
	rng := map[string]any{}
	if fromMS > 0 {
		rng["gte"] = fromMS
	}
	if toMS > 0 {
		rng["lt"] = toMS
	}
	rangeClause := map[string]any{"range": map[string]any{"dtTime": rng}}

	q, _ := root["query"].(map[string]any)
	if q == nil {
		root["query"] = map[string]any{
			"bool": map[string]any{"must": []any{rangeClause}},
		}
		return json.Marshal(root)
	}

	if b, ok := q["bool"].(map[string]any); ok {
		must := stripDtTimeRanges(asSlice(b["must"]))
		filter := stripDtTimeRanges(asSlice(b["filter"]))
		must = append(must, rangeClause)
		b["must"] = must
		if len(filter) > 0 {
			b["filter"] = filter
		} else {
			delete(b, "filter")
		}
		q["bool"] = b
		root["query"] = q
		return json.Marshal(root)
	}

	// Non-bool query: wrap original + dtTime range into bool.must.
	root["query"] = map[string]any{
		"bool": map[string]any{
			"must": []any{q, rangeClause},
		},
	}
	return json.Marshal(root)
}

func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return []any{v}
}

func stripDtTimeRanges(clauses []any) []any {
	if len(clauses) == 0 {
		return nil
	}
	out := make([]any, 0, len(clauses))
	for _, c := range clauses {
		m, ok := c.(map[string]any)
		if !ok {
			out = append(out, c)
			continue
		}
		rng, ok := m["range"].(map[string]any)
		if !ok {
			out = append(out, c)
			continue
		}
		if _, has := rng["dtTime"]; has {
			continue
		}
		out = append(out, c)
	}
	return out
}
