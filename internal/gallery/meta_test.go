package gallery

import "testing"

func TestParseName(t *testing.T) {
	tests := []struct {
		name string
		want NameMeta
	}{
		{
			"Stacked_30_M 42_10.0s_IRCUT_20240101-221530.jpg",
			NameMeta{Subs: 30, SubExposure: 10, Filter: "IR-cut", Date: "2024-01-01"},
		},
		{"M31 S50 Pro 30x10s 2023_12_24.png", NameMeta{Telescope: "S50 Pro", Subs: 30, SubExposure: 10, Date: "2023-12-24"}},
		{"orion_S30_LP.tif", NameMeta{Telescope: "S30", Filter: "LP"}},
		{"holiday-photo.jpg", NameMeta{}},
		{"S500_stack.png", NameMeta{}}, // not a Seestar model name
	}
	for _, tt := range tests {
		if got := ParseName(tt.name); got != tt.want {
			t.Errorf("ParseName(%q) = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestScopeLabel(t *testing.T) {
	for in, want := range map[string]string{
		"S50 Pro_8dc7900b": "S50 Pro",
		"S30":              "S30",
		"Celestron_EdgeHD": "Celestron_EdgeHD", // "EdgeHD" isn't a hex serial
	} {
		if got := ScopeLabel(in); got != want {
			t.Errorf("ScopeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDescribe(t *testing.T) {
	// The file name wins; headers fill the gaps.
	d := Describe("Stacked_30_M 42_10.0s_20240101-221530.jpg", "S50_8dc7900b", "", "2023-05-05T01:02:03", 0)
	want := Details{Telescope: "S50", Date: "2024-01-01", Subs: 30, SubExp: 10, Exposure: 300}
	if d != want {
		t.Errorf("Describe = %+v, want %+v", d, want)
	}
	d = Describe("final.fit", "S30_1234abcd", "LP", "2023-05-05T01:02:03", 1800)
	want = Details{Telescope: "S30", Filter: "LP", Date: "2023-05-05", Exposure: 1800}
	if d != want {
		t.Errorf("Describe (headers only) = %+v, want %+v", d, want)
	}
	if d := Describe("x.png", "", "", "", 0); d != (Details{}) {
		t.Errorf("Describe (nothing known) = %+v", d)
	}
}
