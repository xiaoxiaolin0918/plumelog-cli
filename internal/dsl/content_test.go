package dsl

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeContent(t *testing.T) {
	cases := []struct {
		in    string
		force bool
		want  string
	}{
		{"", false, ""},
		{"timeout", false, "timeout"},
		{"timeout AND retry", false, "timeout AND retry"},
		{"OutOfMemoryError", false, "OutOfMemoryError"},
		{"OutOfMemoryError", true, "*OutOfMemoryError*"},
		{"java.lang.OutOfMemoryError", false, "*java.lang.OutOfMemoryError*"},
		{"java.lang.OutOfMemoryError", true, "*java.lang.OutOfMemoryError*"},
		{"*OutOfMemoryError*", false, "*OutOfMemoryError*"},
		{"*OutOfMemoryError*", true, "*OutOfMemoryError*"},
		{"a.b AND c", false, "a.b AND c"},
		{"field:value.with.dots", false, "field:value.with.dots"},
		{"  java.lang.Error  ", false, "*java.lang.Error*"},
	}
	for _, tc := range cases {
		got := NormalizeContent(tc.in, tc.force)
		if got != tc.want {
			t.Fatalf("NormalizeContent(%q, %v)=%q want %q", tc.in, tc.force, got, tc.want)
		}
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

	body2, err := Build(Search{Content: "OutOfMemoryError", ContentWildcard: true, FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	must2 := mustClauses(t, decode(t, body2))
	qs2 := must2[0].(map[string]any)["query_string"].(map[string]any)
	if qs2["query"] != "*OutOfMemoryError*" {
		t.Fatalf("force wildcard %#v", qs2)
	}

	body3, err := Build(Search{Content: "timeout AND retry", FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	blob, _ := json.Marshal(mustClauses(t, decode(t, body3)))
	if !strings.Contains(string(blob), `"timeout AND retry"`) {
		t.Fatalf("structured query mutated: %s", blob)
	}
}
