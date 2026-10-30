package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Kind is how this copy of Eirin was installed, which decides how it updates.
type Kind string

const (
	KindAppImage Kind = "appimage"
	KindDeb      Kind = "deb"
	KindBinary   Kind = "binary"
	// KindUnsupported: a platform no release is built for.
	KindUnsupported Kind = "unsupported"
)

// Install describes the running installation.
type Install struct {
	Kind Kind
	// Path is the file to replace (AppImage, binary) or the installed
	// executable to relaunch (deb).
	Path string
}

// Detect inspects the running process.
func Detect() Install {
	if runtime.GOOS != "linux" {
		return Install{Kind: KindUnsupported}
	}
	exe, err := os.Executable()
	if err != nil {
		return Install{Kind: KindUnsupported}
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return detectFrom(os.Getenv("APPIMAGE"), exe)
}

// detectFrom classifies an install from the APPIMAGE env var (set by an
// AppImage's AppRun) and the resolved executable path.
func detectFrom(appImage, exe string) Install {
	if appImage != "" {
		if st, err := os.Stat(appImage); err == nil && st.Mode().IsRegular() {
			return Install{Kind: KindAppImage, Path: appImage}
		}
	}
	// Package-managed files live under /usr; /usr/local is a hand-placed binary.
	if strings.HasPrefix(exe, "/usr/") && !strings.HasPrefix(exe, "/usr/local/") {
		return Install{Kind: KindDeb, Path: exe}
	}
	return Install{Kind: KindBinary, Path: exe}
}

// assetName is the release asset to download for this install.
func (i Install) assetName() string {
	ext := map[Kind]string{KindAppImage: "AppImage", KindDeb: "deb", KindBinary: "zip"}[i.Kind]
	return "eirin-" + runtime.GOOS + "-" + runtime.GOARCH + "." + ext
}
