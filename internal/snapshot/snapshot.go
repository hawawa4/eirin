// Package snapshot moves the library catalogue from the desktop app to the
// read-only server viewer through the NAS: the desktop publishes a copy of its
// database to <root>/.eirin/library.db (Publisher), and the server turns that
// into a local, read-only database of final images (Loader).
package snapshot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/TaruDesigns/eirin/internal/store"
)

const (
	// DirName is the hidden folder inside the library root that holds the snapshot.
	DirName  = ".eirin"
	fileName = "library.db"
)

// Path returns where the snapshot for the library at root lives.
func Path(root string) string {
	return filepath.Join(root, DirName, fileName)
}

// Publish writes a consistent copy of st to Path(root). The copy is written
// next to the destination and renamed over it, so readers never see a
// half-written file.
func Publish(st *store.Store, root string) error {
	if root == "" {
		return errors.New("no library root folder set")
	}
	dst := Path(root)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}
	tmp := dst + ".tmp"
	if err := st.VacuumInto(tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}

// removeIfExists deletes path, ignoring a missing file.
func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
