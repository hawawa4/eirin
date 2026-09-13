//go:build !server

package app

import (
	"os/exec"

	"github.com/TaruDesigns/eirin/internal/siril"
)

// OpenProjectInSiril launches Siril with the project folder as working directory.
func (a *App) OpenProjectInSiril(projectFolder string) error {
	exe := a.sirilExecutable()
	cmd := exec.Command(exe, "-d", projectFolder)
	cmd.Env = siril.GUIEnv()
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
