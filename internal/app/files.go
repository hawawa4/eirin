package app

import (
	"fmt"
	"os"
)

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

// BatchRejectFiles marks multiple files as rejected in a single transaction.
func (a *App) BatchRejectFiles(paths []string) error {
	if a.prefs == nil {
		return nil
	}
	return a.prefs.BatchRejectFrames(paths, "")
}

// BatchUnrejectFiles clears the rejection status on multiple files in a single transaction.
func (a *App) BatchUnrejectFiles(paths []string) error {
	if a.prefs == nil {
		return nil
	}
	return a.prefs.BatchUnrejectFrames(paths)
}

// BatchHardDeleteFiles permanently removes multiple files from disk and their frame records.
func (a *App) BatchHardDeleteFiles(paths []string) error {
	var failed []string
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			failed = append(failed, p)
		}
	}
	if a.prefs != nil {
		deletable := make([]string, 0, len(paths)-len(failed))
		failSet := make(map[string]bool, len(failed))
		for _, f := range failed {
			failSet[f] = true
		}
		for _, p := range paths {
			if !failSet[p] {
				deletable = append(deletable, p)
			}
		}
		_ = a.prefs.BatchDeleteFrames(deletable)
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to delete %d file(s)", len(failed))
	}
	return nil
}
