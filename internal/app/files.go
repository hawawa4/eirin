package app

import "os"

// RejectFile marks a file as soft-deleted. It remains on disk but is hidden
// from the normal file listing until explicitly shown in the Rejected view.
func (a *App) RejectFile(path string) error {
	if a.prefs == nil {
		return nil
	}
	return a.prefs.RejectFrame(path, "")
}

// UnrejectFile removes the soft-delete mark, making the file visible again.
func (a *App) UnrejectFile(path string) error {
	if a.prefs == nil {
		return nil
	}
	return a.prefs.UnrejectFrame(path)
}

// HardDeleteFile permanently removes a file from disk and cleans up its frame record.
func (a *App) HardDeleteFile(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}
	if a.prefs != nil {
		_ = a.prefs.DeleteFrame(path)
	}
	return nil
}
