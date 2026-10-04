package app

import "errors"

// errReadOnly is returned by every method that writes to the database or the
// filesystem when running as the headless server viewer.
var errReadOnly = errors.New("the server viewer is read-only")

// requireWritable returns errReadOnly in server builds. Write methods call it
// first, so the bindings stay identical across builds but can't modify
// anything on the server.
func (a *App) requireWritable() error {
	if !desktopModeEnabled {
		return errReadOnly
	}
	return nil
}
