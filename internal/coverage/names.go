package coverage

import (
	"strings"
	"time"
)

// Night is the observing night a DATE-OBS (UTC) belongs to: the date twelve
// hours earlier, so a session that crosses midnight counts once. Empty when
// the timestamp can't be parsed.
func Night(dateObs string) string {
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", time.RFC3339Nano} {
		if t, err := time.Parse(layout, dateObs); err == nil {
			return t.Add(-12 * time.Hour).Format("2006-01-02")
		}
	}
	if len(dateObs) >= 10 {
		return dateObs[:10]
	}
	return ""
}

// ScopeLabel is the display name of a TELESCOP value: Seestar firmware appends
// "_<serial>" ("S50 Pro_8dc7900b"), which is dropped.
func ScopeLabel(scope string) string {
	if scope == "" {
		return "Unknown scope"
	}
	if i := strings.LastIndexByte(scope, '_'); i > 0 && isHex(scope[i+1:]) {
		return scope[:i]
	}
	return scope
}

func isHex(s string) bool {
	if len(s) < 4 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
