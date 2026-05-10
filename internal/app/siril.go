package app

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SirilInfo describes the detected (or user-configured) Siril installation.
type SirilInfo struct {
	Executable string `json:"executable"`
	Version    string `json:"version"`
	Available  bool   `json:"available"`
}

// CheckSiril runs the configured siril executable with -v and returns version info.
func (a *App) CheckSiril() SirilInfo {
	exe := a.sirilExecutable()

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

// SelectSirilExecutable opens a file-picker so the user can locate the siril binary.
func (a *App) SelectSirilExecutable() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Siril Executable",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// SetSirilPath persists a custom siril executable path. Pass an empty string to
// revert to the default PATH lookup.
func (a *App) SetSirilPath(path string) error {
	return a.prefs.Set(prefs.KeySirilPath, path)
}

// OpenWithSiril launches the Siril GUI with the given file, setting the working
// directory to the file's parent folder so Siril's project context is correct.
func (a *App) OpenWithSiril(filePath string) error {
	exe := a.sirilExecutable()
	dir := filepath.Dir(filePath)

	cmd := exec.Command(exe, filepath.Base(filePath))
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (a *App) sirilExecutable() string {
	if p := a.prefs.Load().SirilPath; p != "" {
		return p
	}
	return "siril"
}
