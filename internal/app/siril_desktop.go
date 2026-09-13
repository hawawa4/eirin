//go:build !server

package app

import (
	"os/exec"
	"path/filepath"

	"github.com/TaruDesigns/eirin/internal/siril"
)

// OpenWithSiril launches the Siril GUI with the given file's parent directory
// as the working directory (-d flag). Siril requires -d to set its working dir.
func (a *App) OpenWithSiril(filePath string) error {
	exe := a.sirilExecutable()
	cmd := exec.Command(exe, "-d", filepath.Dir(filePath), filePath)
	cmd.Env = siril.GUIEnv()
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
