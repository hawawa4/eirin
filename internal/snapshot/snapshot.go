// Package snapshot moves the library catalogue from the desktop app to the
// read-only server viewer through the NAS: the desktop publishes a copy of its
// database to <root>/.eirin/library.db (Publisher), and the server turns that
// into a local, read-only database of final images (Loader).
package snapshot

import (
	"errors"
	"fmt"
	"io"
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

// localTempDir is where the snapshot is built before being copied to the
// library; a variable so tests can check nothing is left behind.
var localTempDir = os.TempDir

// Publish writes a consistent copy of st to Path(root).
//
// SQLite builds the copy on local disk: VACUUM INTO locks its destination,
// and file locking doesn't work on network shares (SMB/CIFS reports
// SQLITE_BUSY). The finished file is then copied to the library as plain
// bytes, next to the destination, and renamed over it, so readers never see
// a half-written file.
func Publish(st *store.Store, root string) error {
	if root == "" {
		return errors.New("no library root folder set")
	}
	dst := Path(root)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dst), err)
	}

	local, err := os.CreateTemp(localTempDir(), "eirin-snapshot-*.db")
	if err != nil {
		return fmt.Errorf("create local temp file: %w", err)
	}
	_ = local.Close()
	defer func() { _ = os.Remove(local.Name()) }()
	if err := st.VacuumInto(local.Name()); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	tmp := dst + ".tmp"
	if err := copyFile(local.Name(), tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("copy snapshot to the library: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}

// copyFile copies src to dst (created or truncated) and flushes it to disk.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// removeIfExists deletes path, ignoring a missing file.
func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
