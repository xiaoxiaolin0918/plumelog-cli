package dsl

import (
	"testing"
	"time"
)

func TestParseTimeHumanAndRelative(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 10, 19, 15, 30, 0, 0, loc)

	got, err := ParseTime("now", now, loc)
	if err != nil || !got.Equal(now) {
		t.Fatalf("now: %v %v", got, err)
	}

	got, err = ParseTime("2026-10-16", now, loc)
	want := time.Date(2026, 10, 16, 0, 0, 0, 0, loc)
	if err != nil || !got.Equal(want) {
		t.Fatalf("date-only: %v %v want %v", got, err, want)
	}

	got, err = ParseTime("2026-10-16 00:00", now, loc)
	want = time.Date(2026, 10, 16, 0, 0, 0, 0, loc)
	if err != nil || !got.Equal(want) {
		t.Fatalf("YYYY-MM-DD HH:MM: %v %v", got, err)
	}

	got, err = ParseTime("2026-10-16T00:00:00+08:00", now, loc)
	if err != nil || got.UnixMilli() != want.UnixMilli() {
		t.Fatalf("RFC3339: %v %v", got, err)
	}

	got, err = ParseTime("-3d", now, loc)
	want = now.Add(-3 * 24 * time.Hour)
	if err != nil || !got.Equal(want) {
		t.Fatalf("-3d: %v %v want %v", got, err, want)
	}

	got, err = ParseTime("-2h", now, loc)
	want = now.Add(-2 * time.Hour)
	if err != nil || !got.Equal(want) {
		t.Fatalf("-2h: %v %v want %v", got, err, want)
	}

	got, err = ParseTime("1700000000000", now, loc)
	if err != nil || got.UnixMilli() != 1700000000000 {
		t.Fatalf("ms: %v %v", got, err)
	}

	got, err = ParseTime("1700000000", now, loc)
	if err != nil || got.Unix() != 1700000000 {
		t.Fatalf("seconds: %v %v", got, err)
	}

	if _, err := ParseTime("-0d", now, loc); err == nil {
		t.Fatal("expected error for -0d")
	}
	if _, err := ParseTime("yesterday", now, loc); err == nil {
		t.Fatal("expected error for garbage")
	}
}

func TestResolveWindowRelativeFrom(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, loc)
	from, to, err := ResolveWindow("-2h", "now", time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if to != now.UnixMilli() {
		t.Fatalf("to %d", to)
	}
	if from != now.Add(-2*time.Hour).UnixMilli() {
		t.Fatalf("from %d", from)
	}
}
