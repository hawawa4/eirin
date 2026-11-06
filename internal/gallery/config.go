// Package gallery holds the pure logic of the public gallery (the server
// build): its configuration, the opaque ids images are known by, and the
// metadata worked out from their file names. Nothing here touches the
// database or Wails, so it builds and tests in every configuration.
package gallery

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
)

// Environment variables. Each overrides the same setting in the JSON file.
const (
	EnvConfigFile  = "EIRIN_GALLERY_CONFIG"
	EnvTitle       = "EIRIN_GALLERY_TITLE"
	EnvSubtitle    = "EIRIN_GALLERY_SUBTITLE"
	EnvAbout       = "EIRIN_GALLERY_ABOUT"
	EnvTheme       = "EIRIN_GALLERY_THEME"
	EnvView        = "EIRIN_GALLERY_VIEW"
	EnvFormat      = "EIRIN_GALLERY_FORMAT"
	EnvHomeURL     = "EIRIN_GALLERY_HOME_URL"
	EnvHomeLabel   = "EIRIN_GALLERY_HOME_LABEL"
	defaultTitle   = "Astrophotography"
	defaultView    = ViewGrid
	defaultTheme   = "blue"
	defaultFormat  = FormatAuto
	defaultHomeLbl = "About this project"
)

// Views the gallery can open on.
const (
	ViewGrid      = "grid"
	ViewAtlas     = "atlas"
	ViewSlideshow = "slideshow"
)

// Which rendering of a picture the gallery shows. The library often holds
// every final image twice: the processed FITS and its PNG/JPEG/TIFF export.
const (
	// FormatAuto shows the exports when there are any, else the FITS files.
	FormatAuto   = "auto"
	FormatRaster = "raster"
	FormatFITS   = "fits"
)

// Config is what the gallery's visitors see around the pictures. The JSON
// field names are the file format.
type Config struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	// About is a few sentences (blank lines separate paragraphs), shown on
	// the gallery's front page.
	About string `json:"about"`
	// Theme is the colour theme: blue, red (night) or grey.
	Theme string `json:"theme"`
	// View is the page the gallery opens on: grid, atlas or slideshow.
	View   string `json:"view"`
	Format string `json:"format"`
	// HomeURL adds a link in the header (e.g. back to the project's landing
	// page); HomeLabel is its text.
	HomeURL   string `json:"homeUrl"`
	HomeLabel string `json:"homeLabel"`
}

// Default returns the configuration with nothing set.
func Default() Config {
	return Config{
		Title:     defaultTitle,
		Theme:     defaultTheme,
		View:      defaultView,
		Format:    defaultFormat,
		HomeLabel: defaultHomeLbl,
	}
}

// Load reads the configuration: defaults, then the JSON file (EIRIN_GALLERY_CONFIG,
// or defaultFile when that is unset; a missing defaultFile is fine, a missing
// explicit one is an error), then the environment on top. getenv is os.Getenv
// outside tests.
func Load(getenv func(string) string, defaultFile string) (Config, error) {
	c := Default()

	path, explicit := getenv(EnvConfigFile), true
	if path == "" {
		path, explicit = defaultFile, false
	}
	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &c); err != nil {
				return Config{}, fmt.Errorf("gallery config %s: %w", path, err)
			}
		case explicit || !errors.Is(err, fs.ErrNotExist):
			return Config{}, fmt.Errorf("gallery config: %w", err)
		}
	}

	for env, field := range map[string]*string{
		EnvTitle: &c.Title, EnvSubtitle: &c.Subtitle, EnvAbout: &c.About,
		EnvTheme: &c.Theme, EnvView: &c.View, EnvFormat: &c.Format,
		EnvHomeURL: &c.HomeURL, EnvHomeLabel: &c.HomeLabel,
	} {
		if v := getenv(env); v != "" {
			*field = v
		}
	}
	return c.normalized(), nil
}

// normalized fixes up values that can't be used: unknown choices fall back to
// the defaults, and a link that isn't http(s) or site-relative is dropped (it
// ends up in an href, so "javascript:" must never get through).
func (c Config) normalized() Config {
	d := Default()
	c.Title = strings.TrimSpace(c.Title)
	if c.Title == "" {
		c.Title = d.Title
	}
	c.Subtitle = strings.TrimSpace(c.Subtitle)
	c.About = strings.TrimSpace(c.About)
	c.Theme = strings.ToLower(strings.TrimSpace(c.Theme))
	if c.Theme != "blue" && c.Theme != "red" && c.Theme != "grey" {
		c.Theme = d.Theme
	}
	c.View = strings.ToLower(strings.TrimSpace(c.View))
	if c.View != ViewGrid && c.View != ViewAtlas && c.View != ViewSlideshow {
		c.View = d.View
	}
	c.Format = strings.ToLower(strings.TrimSpace(c.Format))
	if c.Format != FormatAuto && c.Format != FormatRaster && c.Format != FormatFITS {
		c.Format = d.Format
	}
	if !safeLink(c.HomeURL) {
		c.HomeURL = ""
	}
	c.HomeLabel = strings.TrimSpace(c.HomeLabel)
	if c.HomeLabel == "" {
		c.HomeLabel = d.HomeLabel
	}
	return c
}

// safeLink reports whether s is a link the header may point to: an absolute
// http(s) URL, or a path on the same site.
func safeLink(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// ChooseFormat resolves the configured format against what the library
// holds: with FormatAuto, the PNG/JPEG/TIFF exports when there are any (they
// are usually the edited pictures meant for showing), else the FITS files.
func ChooseFormat(mode string, fits, raster int) string {
	switch mode {
	case FormatFITS, FormatRaster:
		return mode
	}
	if raster > 0 {
		return FormatRaster
	}
	return FormatFITS
}
