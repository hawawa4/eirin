//go:build server

package app

import "errors"

// OpenProjectInSiril is unavailable in headless/server builds — launching
// the Siril GUI makes no sense inside a container with no display.
func (a *App) OpenProjectInSiril(projectFolder string) error {
	return errors.New("opening Siril is not available in server mode")
}
