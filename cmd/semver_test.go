package cmd

import "testing"

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		inst, bund string
		want       VersionRelation
	}{
		{"1.2.3", "1.2.3", RelEqual},
		{"v1.2.3", "1.2.3", RelEqual},
		{"0.1.0", "0.2.0", RelOutdated},
		{"0.2.0", "0.1.0", RelAhead},
		{"1.0.0", "0.9.9", RelAhead},
		{"1.0", "1.0.0", RelEqual},
		{"1", "1.0.0", RelEqual},
		{"foo", "1.0.0", RelInvalid},
		{"1.0.0-alpha", "1.0.0", RelInvalid},
		{"", "1.0.0", RelInvalid},
		{"2.0.0", "1.9.9", RelAhead},
		{"1.2.3", "1.2.4", RelOutdated},
	}
	for _, tc := range cases {
		got := CompareSemver(tc.inst, tc.bund)
		if got != tc.want {
			t.Fatalf("CompareSemver(%q,%q)=%q want %q", tc.inst, tc.bund, got, tc.want)
		}
	}
}
