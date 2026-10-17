package dsl

import (
	"encoding/json"
	"testing"
)

func TestEnsureDtTimeRangeAddsToBoolMust(t *testing.T) {
	in := []byte(`{"query":{"bool":{"must":[{"match_all":{}}]}},"sort":[{"dtTime":"desc"}]}`)
	out, err := EnsureDtTimeRange(in, 100, 200)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	must := root["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	if len(must) != 2 {
		t.Fatalf("must=%#v", must)
	}
	dt := must[1].(map[string]any)["range"].(map[string]any)["dtTime"].(map[string]any)
	if int64(dt["gte"].(float64)) != 100 || int64(dt["lt"].(float64)) != 200 {
		t.Fatalf("dtTime=%#v", dt)
	}
}

func TestEnsureDtTimeRangeReplacesExisting(t *testing.T) {
	in := []byte(`{"query":{"bool":{"must":[{"range":{"dtTime":{"gte":1,"lt":2}}}]}}}`)
	out, err := EnsureDtTimeRange(in, 10, 20)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	must := root["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	if len(must) != 1 {
		t.Fatalf("must len %d: %#v", len(must), must)
	}
	dt := must[0].(map[string]any)["range"].(map[string]any)["dtTime"].(map[string]any)
	if int64(dt["gte"].(float64)) != 10 || int64(dt["lt"].(float64)) != 20 {
		t.Fatalf("%#v", dt)
	}
}

func TestEnsureDtTimeRangeWrapsMatchAll(t *testing.T) {
	in := []byte(`{"query":{"match_all":{}}}`)
	out, err := EnsureDtTimeRange(in, 5, 6)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	must := root["query"].(map[string]any)["bool"].(map[string]any)["must"].([]any)
	if len(must) != 2 {
		t.Fatalf("%#v", must)
	}
	if _, ok := must[0].(map[string]any)["match_all"]; !ok {
		t.Fatalf("expected match_all kept: %#v", must[0])
	}
}

func TestEnsureDtTimeRangeStripsFilterDtTime(t *testing.T) {
	in := []byte(`{"query":{"bool":{"filter":[{"range":{"dtTime":{"gte":1}}}]}}}`)
	out, err := EnsureDtTimeRange(in, 9, 10)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	_ = json.Unmarshal(out, &root)
	b := root["query"].(map[string]any)["bool"].(map[string]any)
	if _, ok := b["filter"]; ok {
		t.Fatalf("filter should be gone: %#v", b["filter"])
	}
	must := b["must"].([]any)
	dt := must[0].(map[string]any)["range"].(map[string]any)["dtTime"].(map[string]any)
	if int64(dt["gte"].(float64)) != 9 {
		t.Fatalf("%#v", dt)
	}
}
