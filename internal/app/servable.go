package app

import "errors"

// errNotInLibrary is returned by the server viewer for paths it doesn't serve.
var errNotInLibrary = errors.New("not in the library")

// checkServable limits the server viewer to files in its library (the final
// images of the loaded snapshot); directories and other files are refused.
// The desktop app may read any path.
func (a *App) checkServable(path string) error {
	if desktopModeEnabled {
		return nil
	}
	st := a.store()
	if st == nil {
		return errNotInLibrary
	}
	ok, err := st.HasFrame(path)
	if err != nil {
		return err
	}
	if !ok {
		return errNotInLibrary
	}
	return nil
}
