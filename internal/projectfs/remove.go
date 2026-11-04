package projectfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Remove deletes a project folder and everything in it. Frame symlinks are
// removed without following them, so the NAS frames they point at are never
// touched; copied frames and Siril outputs inside the folder are deleted.
//
// As a guard against a corrupted or stale project record, folder must lie
// strictly inside projectsRoot (the configured projects folder) — never the
// root itself or anywhere else. A folder that is already gone is not an error.
func Remove(projectsRoot, folder string) error {
	if projectsRoot == "" {
		return errors.New("projects folder not configured")
	}
	root, err := filepath.Abs(projectsRoot)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(folder)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("refusing to delete %q: it isn't inside the projects folder %q", folder, projectsRoot)
	}

	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// A project folder that is itself a link: drop the link, not what it points to.
		return os.Remove(target)
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing to delete %q: not a folder", folder)
	}
	return os.RemoveAll(target)
}
