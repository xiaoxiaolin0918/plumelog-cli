package dsl

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeContent(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		force   bool
		noAuto  bool
		want    string
	}{
		{"empty", "", false, false, ""},
		{"plain", "timeout", false, false, "timeout"},
		{"structured AND", "timeout AND retry", false, false, "timeout AND retry"},
		{"force structured AND", "timeout AND retry", true, false, "timeout AND retry"},
		{"bare fragment", "OutOfMemoryError", false, false, "OutOfMemoryError"},
		{"force bare fragment", "OutOfMemoryError", true, false, "*OutOfMemoryError*"},
		{"dotted auto", "java.lang.OutOfMemoryError", false, false, "*java.lang.OutOfMemoryError*"},
		{"dotted force", "java.lang.OutOfMemoryError", true, false, "*java.lang.OutOfMemoryError*"},
		{"dotted no-auto", "java.lang.OutOfMemoryError", false, true, "java.lang.OutOfMemoryError"},
		{"force with no-auto still wraps bare", "OutOfMemoryError", true, true, "*OutOfMemoryError*"},
		{"existing wildcard", "*OutOfMemoryError*", false, false, "*OutOfMemoryError*"},
		{"force existing wildcard", "*OutOfMemoryError*", true, false, "*OutOfMemoryError*"},
		{"dotted AND structured", "a.b AND c", false, false, "a.b AND c"},
		{"force dotted AND structured", "a.b AND c", true, false, "a.b AND c"},
		{"field:value structured", "field:value.with.dots", false, false, "field:value.with.dots"},
		{"force field:value structured", "field:value.with.dots", true, false, "field:value.with.dots"},
		{"trim dotted", "  java.lang.Error  ", false, false, "*java.lang.Error*"},
		{"quoted phrase structured", `"exact.phrase"`, true, false, `"exact.phrase"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeContent(tc.in, tc.force, tc.noAuto)
			if got != tc.want {
				t.Fatalf("NormalizeContent(%q, force=%v, noAuto=%v)=%q want %q", tc.in, tc.force, tc.noAuto, got, tc.want)
			}
		})
	}
}

func TestIsStructuredQueryString(t *testing.T) {
	structured := []string{
		"timeout AND retry",
		"a OR b",
		"NOT foo",
		"field:value.with.dots",
		`"quoted"`,
		"has space",
		"a&&b",
		"a||b",
	}
	for _, s := range structured {
		if !isStructuredQueryString(s) {
			t.Fatalf("expected structured: %q", s)
		}
		if NormalizeContent(s, true, false) != s {
			t.Fatalf("force must not wrap structured %q", s)
		}
	}
	if isStructuredQueryString("OutOfMemoryError") {
		t.Fatal("bare fragment should not be structured")
	}
	if isStructuredQueryString("java.lang.OutOfMemoryError") {
		t.Fatal("dotted identifier should not be structured")
	}
}

func TestBuildContentWildcardAndDottedAuto(t *testing.T) {
	body, err := Build(Search{Content: "java.lang.OutOfMemoryError", FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	must := mustClauses(t, decode(t, body))
	qs := must[0].(map[string]any)["query_string"].(map[string]any)
	if qs["query"] != "*java.lang.OutOfMemoryError*" {
		t.Fatalf("auto dotted %#v", qs)
	}

	bodyNo, err := Build(Search{Content: "java.lang.OutOfMemoryError", NoContentAutoWildcard: true, FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	qsNo := mustClauses(t, decode(t, bodyNo))[0].(map[string]any)["query_string"].(map[string]any)
	if qsNo["query"] != "java.lang.OutOfMemoryError" {
		t.Fatalf("no-auto %#v", qsNo)
	}

	body2, err := Build(Search{Content: "OutOfMemoryError", ContentWildcard: true, FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	qs2 := mustClauses(t, decode(t, body2))[0].(map[string]any)["query_string"].(map[string]any)
	if qs2["query"] != "*OutOfMemoryError*" {
		t.Fatalf("force wildcard %#v", qs2)
	}

	for _, q := range []string{"timeout AND retry", "field:value.with.dots"} {
		body3, err := Build(Search{Content: q, ContentWildcard: true, FromMS: 1, ToMS: 2})
		if err != nil {
			t.Fatal(err)
		}
		blob, _ := json.Marshal(mustClauses(t, decode(t, body3)))
		if !strings.Contains(string(blob), jsonQuote(q)) {
			t.Fatalf("structured force mutated %q: %s", q, blob)
		}
		if strings.Contains(string(blob), "*"+q+"*") {
			t.Fatalf("structured force wrapped %q: %s", q, blob)
		}
	}
}

func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}