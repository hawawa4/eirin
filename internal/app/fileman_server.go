//go:build server

package app

import "errors"

// RevealPath is unavailable in headless/server builds — there is no desktop
// file manager to open.
func (a *App) RevealPath(path string) error {
	return errors.New("not available in server mode")
}

// OpenFolder is unavailable in headless/server builds. See RevealPath.
func (a *App) OpenFolder(path string) error {
	return errors.New("not available in server mode")
}
