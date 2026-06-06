package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/TaruDesigns/eirin/internal/store"
)

// RejectFile marks a file as soft-deleted. It remains on disk but is hidden
// from the normal file listing until explicitly shown in the Rejected view.
func (a *App) RejectFile(path string) error {
	if a.store == nil {
		return nil
	}
	return a.store.RejectFrame(path, "")
}

// UnrejectFile removes the soft-delete mark, making the file visible again.
func (a *App) UnrejectFile(path string) error {
	if a.store == nil {
		return nil
	}
	return a.store.UnrejectFrame(path)
}

// HardDeleteFile permanently removes a file from disk and cleans up its frame record.
func (a *App) HardDeleteFile(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}
	if a.store != nil {
		_ = a.store.DeleteFrame(path)
	}
	return nil
}

// BatchRejectFiles marks multiple files as rejected in a single transaction.
func (a *App) BatchRejectFiles(paths []string) error {
	if a.store == nil {
		return nil
	}
	return a.store.BatchRejectFrames(paths, "")
}

// BatchUnrejectFiles clears the rejection status on multiple files in a single transaction.
func (a *App) BatchUnrejectFiles(paths []string) error {
	if a.store == nil {
		return nil
	}
	return a.store.BatchUnrejectFrames(paths)
}

// RenameFrame renames a file on disk and updates the DB key accordingly.
// The new name must be in the same directory as the old path.
func (a *App) RenameFrame(oldPath, newName string) (string, error) {
	dir := filepath.Dir(oldPath)
	newPath := filepath.Join(dir, newName)
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("rename file: %w", err)
	}
	if a.store != nil {
		if err := a.store.RenameFrame(oldPath, newPath); err != nil {
			return newPath, fmt.Errorf("rename db record: %w", err)
		}
	}
	return newPath, nil
}

// UpdateFrameMeta writes user-supplied metadata overrides into the DB.
// Empty fields are ignored — they leave the existing value unchanged.
func (a *App) UpdateFrameMeta(path string, meta store.FrameMeta) error {
	if a.store == nil {
		return nil
	}
	return a.store.UpdateFrameMeta(path, meta)
}

// BatchHardDeleteFiles permanently removes multiple files from disk and their frame records.
func (a *App) BatchHardDeleteFiles(paths []string) error {
	var failed []string
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			failed = append(failed, p)
		}
	}
	if a.store != nil {
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
		_ = a.store.BatchDeleteFrames(deletable)
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to delete %d file(s)", len(failed))
	}
	return nil
}
