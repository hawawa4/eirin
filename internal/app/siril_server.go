//go:build server

package app

import "errors"

// OpenWithSiril is unavailable in headless/server builds — launching the
// Siril GUI makes no sense inside a container with no display.
func (a *App) OpenWithSiril(filePath string) error {
	return errors.New("opening Siril is not available in server mode")
}
