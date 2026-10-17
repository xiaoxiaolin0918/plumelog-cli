package dsl

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// indexDateRE matches a YYYYMMDD calendar day, usually as a trailing _YYYYMMDD
// on PlumeLog daily indices (plume_log_run_20261016 / plume_log_trace_20261016).
var indexDateRE = regexp.MustCompile(`(?:^|_)((?:19|20)\d{6})(?:$|_)`)

// ParseIndexDate extracts the calendar day embedded in an index name.
// ok is false when no valid YYYYMMDD is found.
func ParseIndexDate(index string, loc *time.Location) (day time.Time, ok bool) {
	if loc == nil {
		loc = time.Local
	}
	s := strings.TrimSpace(index)
	if s == "" {
		return time.Time{}, false
	}
	// Prefer the rightmost match so a trailing _YYYYMMDD wins.
	matches := indexDateRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return time.Time{}, false
	}
	raw := matches[len(matches)-1][1]
	t, err := time.ParseInLocation("20060102", raw, loc)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc), true
}

// IndexDayWindow returns [start, end) unix-ms for the full calendar day of the
// date embedded in index. end is exclusive (dtTime lt semantics).
func IndexDayWindow(index string, loc *time.Location) (fromMS, toMS int64, day time.Time, ok bool) {
	day, ok = ParseIndexDate(index, loc)
	if !ok {
		return 0, 0, time.Time{}, false
	}
	end := day.Add(24 * time.Hour)
	return day.UnixMilli(), end.UnixMilli(), day, true
}

// FormatIndexDayNote is a short stderr note when CLI auto-aligns the window.
func FormatIndexDayNote(index string, day time.Time) string {
	return fmt.Sprintf("note: --index %s without --from-time/--to-time/--range; using index day %s (full day)\n",
		index, day.Format("2006-01-02"))
}