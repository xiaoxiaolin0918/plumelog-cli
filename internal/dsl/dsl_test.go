package dsl

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildShortcutFilters(t *testing.T) {
	body, err := Build(Search{
		App:        "order,api",
		Env:        "prod,blue",
		Level:      "ERROR",
		TraceID:    "",
		ClassName:  "a.b.C",
		ServerName: "10.0.0.8",
		Content:    "timeout AND retry",
		Extend:     []string{"userId=42"},
		FromMS:     1700000000000,
		ToMS:       1700003600000,
		Highlight:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	root := decode(t, body)
	must := mustClauses(t, root)
	if got := phraseOf(t, must[0], "appName"); got != "order api" {
		t.Fatalf("app phrase %q", got)
	}
	if got := phraseOf(t, must[1], "env"); got != "prod,blue" {
		t.Fatalf("env phrase %q", got)
	}
	if got := phraseOf(t, must[2], "logLevel"); got != "ERROR" {
		t.Fatalf("level %q", got)
	}
	content := must[len(must)-2].(map[string]any)
	if _, ok := content["query_string"]; !ok {
		t.Fatalf("content clause = %#v", content)
	}
	qs := content["query_string"].(map[string]any)
	if qs["default_field"] != "content" || qs["query"] != "timeout AND retry" {
		t.Fatalf("query_string %#v", qs)
	}
	rng := must[len(must)-1].(map[string]any)["range"].(map[string]any)["dtTime"].(map[string]any)
	if rng["gte"] != float64(1700000000000) || rng["lt"] != float64(1700003600000) {
		t.Fatalf("range %#v", rng)
	}
	sort := root["sort"].([]any)
	if len(sort) != 1 || sort[0].(map[string]any)["dtTime"] != "desc" {
		t.Fatalf("sort %#v", sort)
	}
	frag := root["highlight"].(map[string]any)["fields"].(map[string]any)["content"].(map[string]any)["fragment_size"]
	if frag != float64(2147483647) {
		t.Fatalf("fragment %v", frag)
	}
}

func TestBuildTraceSortAndExclude(t *testing.T) {
	body, err := Build(Search{
		App:        "a,b",
		ExcludeApp: true,
		TraceID:    "tid-1",
		Highlight:  false,
	})
	if err != nil {
		t.Fatal(err)
	}
	root := decode(t, body)
	if _, ok := root["highlight"]; ok {
		t.Fatal("highlight should be omitted")
	}
	boolQ := root["query"].(map[string]any)["bool"].(map[string]any)
	if _, ok := boolQ["must"]; !ok {
		// traceId is still a must
	}
	must := boolQ["must"].([]any)
	if phraseOf(t, must[0], "traceId") != "tid-1" {
		t.Fatalf("must %#v", must)
	}
	not := boolQ["must_not"].([]any)
	if len(not) != 2 {
		t.Fatalf("must_not %#v", not)
	}
	if phraseOf(t, not[0], "appName") != "a" || phraseOf(t, not[1], "appName") != "b" {
		t.Fatalf("must_not %#v", not)
	}
	sort := root["sort"].([]any)
	if sort[0].(map[string]any)["dtTime"] != "asc" || sort[1].(map[string]any)["seq"] != "asc" {
		t.Fatalf("sort %#v", sort)
	}
}

func TestResolveWindowClampsFuture(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	from, to, err := ResolveWindow("", "2099-01-01T00:00:00Z", time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if to != now.UnixMilli() {
		t.Fatalf("to %d want %d", to, now.UnixMilli())
	}
	if from != now.Add(-time.Hour).UnixMilli() {
		t.Fatalf("from %d", from)
	}
	got, err := ParseTime("1700000000000", now, time.UTC)
	if err != nil || got.UnixMilli() != 1700000000000 {
		t.Fatalf("ms parse %v %v", got, err)
	}
}

func TestExcludeRequiresApp(t *testing.T) {
	if _, err := Build(Search{ExcludeApp: true}); err == nil {
		t.Fatal("expected error")
	}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func mustClauses(t *testing.T, root map[string]any) []any {
	t.Helper()
	return root["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
}

func phraseOf(t *testing.T, clause any, field string) string {
	t.Helper()
	mp := clause.(map[string]any)["match_phrase"].(map[string]any)
	inner := mp[field].(map[string]any)
	return inner["query"].(string)
}
