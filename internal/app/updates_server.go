//go:build server

package app

import "errors"

var errNoUpdates = errors.New("not available in server mode")

// updateSide is empty: the headless server is updated by pulling a new image.
type updateSide struct{}

// GetAppVersion is meaningless for the server viewer.
func (a *App) GetAppVersion() string {
	return ""
}

// CheckForUpdate is unavailable in headless/server builds.
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	return UpdateInfo{}, errNoUpdates
}

// InstallUpdate is unavailable in headless/server builds.
func (a *App) InstallUpdate() error {
	return errReadOnly
}

// RestartApp is unavailable in headless/server builds.
func (a *App) RestartApp() error {
	return errReadOnly
}

// OpenReleasePage is unavailable in headless/server builds.
func (a *App) OpenReleasePage() error {
	return errNoUpdates
}
