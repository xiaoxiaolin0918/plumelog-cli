package dsl

import "testing"

func TestBuildQueryStringDefaultOperatorAND(t *testing.T) {
	body, err := Build(Search{Content: "Column index out of range", FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	qs := mustClauses(t, decode(t, body))[0].(map[string]any)["query_string"].(map[string]any)
	if qs["query"] != "Column index out of range" {
		t.Fatalf("query %#v", qs["query"])
	}
	if qs["default_field"] != "content" {
		t.Fatalf("default_field %#v", qs["default_field"])
	}
	if qs["default_operator"] != "AND" {
		t.Fatalf("default_operator %#v want AND", qs["default_operator"])
	}

	// Explicit Lucene OR still preserved in query text; default_operator only affects bare spaces.
	bodyOR, err := Build(Search{Content: "foo OR bar", FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	qsOR := mustClauses(t, decode(t, bodyOR))[0].(map[string]any)["query_string"].(map[string]any)
	if qsOR["query"] != "foo OR bar" || qsOR["default_operator"] != "AND" {
		t.Fatalf("explicit OR case %#v", qsOR)
	}

	// Quoted phrase unchanged; still gets AND operator (harmless for phrases).
	bodyQ, err := Build(Search{Content: `"exact phrase"`, FromMS: 1, ToMS: 2})
	if err != nil {
		t.Fatal(err)
	}
	qsQ := mustClauses(t, decode(t, bodyQ))[0].(map[string]any)["query_string"].(map[string]any)
	if qsQ["query"] != `"exact phrase"` || qsQ["default_operator"] != "AND" {
		t.Fatalf("quoted %#v", qsQ)
	}
}
