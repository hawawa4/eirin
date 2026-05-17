package siril

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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

	version := strings.TrimSpace(buf.String())
	if version == "" {
		version = "(no output)"
	}
	return SirilInfo{Executable: exe, Version: version, Available: true}
}
