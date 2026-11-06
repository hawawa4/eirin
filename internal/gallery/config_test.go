package gallery

import (
	"os"
	"path/filepath"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gallery.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(nil), "")
	if err != nil {
		t.Fatal(err)
	}
	if c != Default() {
		t.Errorf("got %+v, want defaults %+v", c, Default())
	}
}

func TestLoadFileThenEnv(t *testing.T) {
	p := writeConfig(t, `{"title":"From file","subtitle":"Sub","theme":"red","view":"atlas"}`)
	c, err := Load(env(map[string]string{EnvConfigFile: p, EnvTitle: "From env"}), "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "From env" {
		t.Errorf("env must win over the file: title = %q", c.Title)
	}
	if c.Subtitle != "Sub" || c.Theme != "red" || c.View != ViewAtlas {
		t.Errorf("file values lost: %+v", c)
	}
}

func TestLoadMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.json")
	if _, err := Load(env(nil), missing); err != nil {
		t.Errorf("a missing default file is fine, got %v", err)
	}
	if _, err := Load(env(map[string]string{EnvConfigFile: missing}), ""); err == nil {
		t.Error("a missing explicit file must be an error")
	}
}

func TestLoadBadJSON(t *testing.T) {
	p := writeConfig(t, `{"title":`)
	if _, err := Load(env(map[string]string{EnvConfigFile: p}), ""); err == nil {
		t.Error("invalid JSON must be an error")
	}
}

func TestNormalized(t *testing.T) {
	c, err := Load(env(map[string]string{
		EnvTheme: "PURPLE", EnvView: "library", EnvFormat: "tiff", EnvTitle: "   ",
		EnvHomeURL: "javascript:alert(1)",
	}), "")
	if err != nil {
		t.Fatal(err)
	}
	d := Default()
	if c.Theme != d.Theme || c.View != d.View || c.Format != d.Format || c.Title != d.Title {
		t.Errorf("invalid choices must fall back to the defaults: %+v", c)
	}
	if c.HomeURL != "" {
		t.Errorf("javascript: link kept: %q", c.HomeURL)
	}
}

func TestSafeLink(t *testing.T) {
	for link, want := range map[string]bool{
		"https://eirin.hawawa.org": true,
		"http://example.com/a":     true,
		"/about":                   true,
		"//evil.example":           false,
		"javascript:alert(1)":      false,
		"data:text/html,x":         false,
		"mailto:a@b.c":             false,
		"https://":                 false,
		"":                         false,
	} {
		if got := safeLink(link); got != want {
			t.Errorf("safeLink(%q) = %v, want %v", link, got, want)
		}
	}
}

func TestChooseFormat(t *testing.T) {
	tests := []struct {
		mode         string
		fits, raster int
		want         string
	}{
		{FormatAuto, 5, 3, FormatRaster},
		{FormatAuto, 5, 0, FormatFITS},
		{FormatAuto, 0, 0, FormatFITS},
		{FormatFITS, 5, 3, FormatFITS},
		{FormatRaster, 5, 0, FormatRaster},
	}
	for _, tt := range tests {
		if got := ChooseFormat(tt.mode, tt.fits, tt.raster); got != tt.want {
			t.Errorf("ChooseFormat(%q, %d, %d) = %q, want %q", tt.mode, tt.fits, tt.raster, got, tt.want)
		}
	}
}
