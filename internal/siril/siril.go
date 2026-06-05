package siril

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var sirilVersionRe = regexp.MustCompile(`siril\s+(\S+)`)

func parseSirilVersion(output string) string {
	if m := sirilVersionRe.FindStringSubmatch(output); m != nil {
		return m[1]
	}
	return "(unknown)"
}

// GUIEnv returns an environment suitable for launching Siril's Qt GUI as a
// child process. Wails forces GDK_BACKEND=x11 (and sometimes QT_QPA_PLATFORM)
// on Wayland so that WebKit works, but those overrides break Qt child processes.
// We start from the full process environment and strip the Qt/GDK platform
// overrides so Qt can auto-detect Wayland (or X11) on its own.
func GUIEnv() []string {
	env := os.Environ()
	filtered := env[:0]
	for _, e := range env {
		switch {
		case strings.HasPrefix(e, "QT_QPA_PLATFORM="),
			strings.HasPrefix(e, "GDK_BACKEND="):
			// drop — let Qt/GDK detect the display server themselves
		default:
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// SirilInfo describes the detected (or user-configured) Siril installation.
type SirilInfo struct {
	Executable string `json:"executable"`
	Version    string `json:"version"`
	Available  bool   `json:"available"`
}

// Executable returns the siril executable path: configured path if non-empty,
// otherwise "siril" (expected to be on PATH).
func Executable(configuredPath string) string {
	if configuredPath != "" {
		return configuredPath
	}
	return "siril"
}

// CLIExecutable returns the best available headless Siril binary.
// If configuredPath is set and a siril-cli sibling exists, it is preferred.
func CLIExecutable(configuredPath string) string {
	if configuredPath != "" {
		cliPath := filepath.Join(filepath.Dir(configuredPath), "siril-cli")
		if _, err := os.Stat(cliPath); err == nil {
			return cliPath
		}
		return configuredPath
	}
	if p, err := exec.LookPath("siril-cli"); err == nil {
		return p
	}
	return "siril"
}

// CheckSiril runs the given siril executable with -v and returns version info.
func CheckSiril(exe string) SirilInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, "-v")
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return SirilInfo{Executable: exe, Version: "not found", Available: false}
	}

	version := parseSirilVersion(buf.String())
	return SirilInfo{Executable: exe, Version: version, Available: true}
}
