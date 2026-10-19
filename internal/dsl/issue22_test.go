package dsl

import (
	"testing"
	"time"
)

func TestRangeWindow(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"day", 24 * time.Hour},
		{"week", 7 * 24 * time.Hour},
		{"month", 30 * 24 * time.Hour},
		{"WEEK", 7 * 24 * time.Hour},
	}
	for _, tc := range cases {
		got, err := RangeWindow(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("RangeWindow(%q)=%v %v want %v", tc.in, got, err, tc.want)
		}
	}
	if _, err := RangeWindow("year"); err == nil {
		t.Fatal("expected error for year")
	}
}

func TestResolveWindowWithRangeDuration(t *testing.T) {
	now := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	week, err := RangeWindow("week")
	if err != nil {
		t.Fatal(err)
	}
	from, to, err := ResolveWindow("", "", week, now)
	if err != nil {
		t.Fatal(err)
	}
	if to != now.UnixMilli() {
		t.Fatalf("to %d", to)
	}
	wantFrom := now.Add(-week).UnixMilli()
	if from != wantFrom {
		t.Fatalf("from %d want %d", from, wantFrom)
	}
}
