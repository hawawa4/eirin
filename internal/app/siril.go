package app

import (
	"github.com/TaruDesigns/eirin/internal/siril"
	"github.com/TaruDesigns/eirin/internal/store"
)

// SirilInfo describes the detected (or user-configured) Siril installation.
// Re-exported from the siril package for Wails binding compatibility.
type SirilInfo = siril.SirilInfo

// CheckSiril runs the configured siril executable with -v and returns version info.
func (a *App) CheckSiril() SirilInfo {
	exe := a.sirilExecutable()
	return siril.CheckSiril(exe)
}

// SelectSirilExecutable opens a file-picker so the user can locate the siril binary.
func (a *App) SelectSirilExecutable() (string, error) {
	path, err := a.wails.Dialog.OpenFile().
		SetTitle("Select Siril Executable").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

// SetSirilPath persists a custom siril executable path. Pass an empty string to
// revert to the default PATH lookup.
func (a *App) SetSirilPath(path string) error {
	return a.store.Set(store.KeySirilPath, path)
}

func (a *App) sirilExecutable() string {
	return siril.Executable(a.store.Load().SirilPath)
}

func (a *App) sirilCliExecutable() string {
	return siril.CLIExecutable(a.store.Load().SirilPath)
}
