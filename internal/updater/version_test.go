package updater

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.6.1", "1.6.0", true},
		{"v1.7.0", "1.6.9", true},
		{"2.0.0", "1.99.99", true},
		{"1.6.0", "1.6.0", false},
		{"1.5.0", "1.6.0", false},
		{"1.10.0", "1.9.0", true}, // numeric, not lexical
		{"1.6.0", "dev", false},
		{"nightly", "1.6.0", false},
		{"1.6.0-rc1", "1.5.0", false},
		{"1.06.0", "1.5.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	if !IsRelease("1.2.3") || !IsRelease("v1.2.3") {
		t.Error("expected release versions")
	}
	if IsRelease("dev") || IsRelease("1.2") || IsRelease("1.2.3.4") {
		t.Error("expected non-release versions")
	}
}
