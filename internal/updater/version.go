// Package updater checks GitHub releases for a newer Eirin and installs it in
// place: AppImage and bare-binary installs replace themselves, .deb installs
// hand the downloaded package to apt through pkexec.
package updater

import (
	"strconv"
	"strings"
)

// Current is the running version ("1.6.0"). Release builds set it with
// -ldflags "-X github.com/hawawa4/eirin/internal/updater.Current=1.6.0";
// anything else (dev runs, local builds) stays "dev" and is never updated.
var Current = "dev"

// parseVersion reads "1.2.3" or "v1.2.3". Anything else (pre-release
// suffixes, "dev") is not a release version.
func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(s), "v"), ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p != strconv.Itoa(n) {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// IsRelease reports whether v is a release version number.
func IsRelease(v string) bool {
	_, ok := parseVersion(v)
	return ok
}

// Newer reports whether latest is a higher release version than current. It is
// false when either side isn't a release version.
func Newer(latest, current string) bool {
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}
