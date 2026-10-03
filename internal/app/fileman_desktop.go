//go:build !server

package app

import "github.com/TaruDesigns/eirin/internal/fileman"

// RevealPath opens the OS file manager with path selected.
func (a *App) RevealPath(path string) error {
	return fileman.Reveal(path)
}

// OpenFolder opens path (or, for a file, its parent folder) in the OS file manager.
func (a *App) OpenFolder(path string) error {
	return fileman.OpenDir(path)
}
