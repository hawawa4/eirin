package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFrom(t *testing.T) {
	appImage := filepath.Join(t.TempDir(), "Eirin.AppImage")
	if err := os.WriteFile(appImage, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		appImage string
		exe      string
		want     Install
	}{
		{"appimage", appImage, "/tmp/.mount_x/usr/bin/eirin", Install{KindAppImage, appImage}},
		{"stale APPIMAGE var", "/does/not/exist", "/home/u/bin/eirin", Install{KindBinary, "/home/u/bin/eirin"}},
		{"deb", "", "/usr/bin/eirin", Install{KindDeb, "/usr/bin/eirin"}},
		{"hand-placed in /usr/local", "", "/usr/local/bin/eirin", Install{KindBinary, "/usr/local/bin/eirin"}},
		{"unpacked zip", "", "/home/u/eirin", Install{KindBinary, "/home/u/eirin"}},
	}
	for _, c := range cases {
		if got := detectFrom(c.appImage, c.exe); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}
