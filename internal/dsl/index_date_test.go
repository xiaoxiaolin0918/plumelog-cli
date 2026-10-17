package dsl

import (
	"testing"
	"time"
)

func TestParseIndexDate(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	cases := []struct {
		index string
		want  string
		ok    bool
	}{
		{"plume_log_run_20261016", "2026-10-16", true},
		{"plume_log_trace_20240101", "2024-01-01", true},
		{"plume_log_run_20261016_extra", "2026-10-16", true},
		{"no-date-here", "", false},
		{"", "", false},
		{"plume_log_run_20261399", "", false},
	}
	for _, tc := range cases {
		day, ok := ParseIndexDate(tc.index, loc)
		if ok != tc.ok {
			t.Fatalf("%q ok=%v want %v", tc.index, ok, tc.ok)
		}
		if !tc.ok {
			continue
		}
		if got := day.Format("2006-01-02"); got != tc.want {
			t.Fatalf("%q day=%s want %s", tc.index, got, tc.want)
		}
		if day.Location() != loc {
			t.Fatalf("location")
		}
	}
}

func TestIndexDayWindow(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	from, to, day, ok := IndexDayWindow("plume_log_run_20261016", loc)
	if !ok {
		t.Fatal("expected ok")
	}
	if day.Format("2006-01-02") != "2026-10-16" {
		t.Fatalf("day %v", day)
	}
	wantFrom := time.Date(2026, 10, 16, 0, 0, 0, 0, loc).UnixMilli()
	wantTo := time.Date(2026, 10, 17, 0, 0, 0, 0, loc).UnixMilli()
	if from != wantFrom || to != wantTo {
		t.Fatalf("window %d-%d want %d-%d", from, to, wantFrom, wantTo)
	}
}