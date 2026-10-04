package snapshot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/TaruDesigns/eirin/internal/store"
)

// DefaultPollInterval is how often the loader checks the NAS for a new snapshot.
const DefaultPollInterval = time.Minute

// retireDelay is how long a replaced store stays open, so requests that
// started on it can finish.
const retireDelay = 30 * time.Second

// Metadata written into the viewer database, so a restarted server knows
// which snapshot its local copy came from.
const (
	metaSourceRoot = "snapshot_source_root"
	metaModTime    = "snapshot_mod_time" // snapshot file mtime, unix nanoseconds
	metaSize       = "snapshot_size"
)

// LoaderStatus describes the server side of the snapshot.
type LoaderStatus struct {
	Root        string    // library root on this server (EIRIN_ROOT)
	Loaded      bool      // a snapshot has been loaded
	SourceRoot  string    // library root on the desktop that published it
	PublishedAt time.Time // when the loaded snapshot was written
	LoadedAt    time.Time // when this server loaded it
	Frames      int
	LastError   string
}

// Loader keeps a local, read-only viewer database in sync with the snapshot
// the desktop app publishes under root. Each new snapshot is copied locally,
// its paths are rewritten to root, it is pruned to final images, and the
// result is handed to swap; the store swap returns is closed shortly after.
type Loader struct {
	root     string
	local    string
	swap     func(*store.Store) *store.Store
	interval time.Duration

	mu       sync.Mutex
	status   LoaderStatus
	seenMod  time.Time // snapshot file version already handled (loaded or failed)
	seenSize int64
}

// NewLoader returns a loader for the library at root, keeping its copy at local.
func NewLoader(root, local string, swap func(*store.Store) *store.Store) *Loader {
	root = filepath.Clean(root)
	return &Loader{
		root:     root,
		local:    local,
		swap:     swap,
		interval: DefaultPollInterval,
		status:   LoaderStatus{Root: root},
	}
}

// Open returns the store to serve at startup: the local copy from a previous
// run if it matches root, otherwise an empty viewer database.
func (l *Loader) Open() (*store.Store, error) {
	if st, err := store.OpenReadOnly(l.local); err == nil {
		if root, _ := st.Get(store.KeyRootFolder); root == l.root {
			l.mu.Lock()
			l.recordLoaded(st, time.Time{})
			l.mu.Unlock()
			return st, nil
		}
		// Copy made for a different mount point: start over.
		_ = st.Close()
	}
	if err := l.build(l.local, func(st *store.Store) error {
		return st.Set(store.KeyRootFolder, l.root)
	}); err != nil {
		return nil, fmt.Errorf("create empty viewer database: %w", err)
	}
	return store.OpenReadOnly(l.local)
}

// Run checks for a new snapshot every interval until ctx is cancelled.
func (l *Loader) Run(ctx context.Context) {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()
	for {
		l.Check()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Check loads the snapshot if it changed since the last check. It reports
// whether a new store was swapped in.
func (l *Loader) Check() bool {
	fi, err := os.Stat(Path(l.root))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			l.setError(err)
		}
		return false
	}
	l.mu.Lock()
	seen := fi.ModTime().Equal(l.seenMod) && fi.Size() == l.seenSize
	l.mu.Unlock()
	if seen {
		return false
	}

	st, err := l.load(fi)
	l.mu.Lock()
	// Don't retry a broken snapshot every poll; wait for the next publish.
	l.seenMod, l.seenSize = fi.ModTime(), fi.Size()
	l.mu.Unlock()
	if err != nil {
		l.setError(err)
		return false
	}

	l.mu.Lock()
	l.recordLoaded(st, time.Now())
	l.mu.Unlock()
	if old := l.swap(st); old != nil {
		time.AfterFunc(retireDelay, func() { _ = old.Close() })
	}
	slog.Info("snapshot: loaded", "frames", l.Status().Frames)
	return true
}

// Status returns the current state for the settings UI.
func (l *Loader) Status() LoaderStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status
}

// load turns the published snapshot described by fi into a new local
// read-only store.
func (l *Loader) load(fi fs.FileInfo) (*store.Store, error) {
	tmp := l.local + ".tmp"
	if err := copySnapshot(Path(l.root), tmp, fi); err != nil {
		return nil, err
	}
	err := l.prepare(tmp, func(st *store.Store) error {
		v, err := st.SchemaVersion()
		if err != nil {
			return err
		}
		if v > store.LatestSchemaVersion() {
			return fmt.Errorf("snapshot is from a newer version of Eirin (schema %d, this server knows %d): update the server",
				v, store.LatestSchemaVersion())
		}
		src, _ := st.Get(store.KeyRootFolder)
		if src == "" {
			return errors.New("snapshot has no library root folder")
		}
		if err := st.RewriteRoot(src, l.root); err != nil {
			return err
		}
		if err := st.PruneToFinals(); err != nil {
			return err
		}
		for k, v := range map[string]string{
			metaSourceRoot: src,
			metaModTime:    strconv.FormatInt(fi.ModTime().UnixNano(), 10),
			metaSize:       strconv.FormatInt(fi.Size(), 10),
		} {
			if err := st.Set(k, v); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = removeDB(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, l.local); err != nil {
		_ = removeDB(tmp)
		return nil, err
	}
	return store.OpenReadOnly(l.local)
}

// prepare opens the database at path (migrating it forward), applies fn, and
// compacts it into a file OpenReadOnly can serve.
func (l *Loader) prepare(path string, fn func(*store.Store) error) error {
	st, err := store.NewStoreAt(path)
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		_ = st.Close()
		return err
	}
	if err := st.Compact(); err != nil {
		_ = st.Close()
		return err
	}
	return st.Close()
}

// build creates a fresh database at dst through a temp file.
func (l *Loader) build(dst string, fn func(*store.Store) error) error {
	tmp := dst + ".tmp"
	if err := removeDB(tmp); err != nil {
		return err
	}
	if err := l.prepare(tmp, fn); err != nil {
		_ = removeDB(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// recordLoaded fills the status from st's metadata. Callers hold l.mu.
func (l *Loader) recordLoaded(st *store.Store, loadedAt time.Time) {
	s := LoaderStatus{Root: l.root, LoadedAt: loadedAt}
	if n, err := st.FrameCount(); err == nil {
		s.Frames = n
	}
	src, ok := st.Get(metaSourceRoot)
	if ok {
		s.Loaded = true
		s.SourceRoot = src
		mod, _ := st.Get(metaModTime)
		size, _ := st.Get(metaSize)
		if ns, err := strconv.ParseInt(mod, 10, 64); err == nil {
			s.PublishedAt = time.Unix(0, ns)
			l.seenMod = s.PublishedAt
		}
		if n, err := strconv.ParseInt(size, 10, 64); err == nil {
			l.seenSize = n
		}
	}
	l.status = s
}

func (l *Loader) setError(err error) {
	slog.Warn("snapshot: load failed", "err", err)
	l.mu.Lock()
	l.status.LastError = err.Error()
	l.mu.Unlock()
}

// copySnapshot copies the published snapshot to dst and checks it wasn't
// replaced mid-copy.
func copySnapshot(src, dst string, before fs.FileInfo) error {
	if err := removeDB(dst); err != nil {
		return err
	}
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
		return fmt.Errorf("copy snapshot: %w", err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	after, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		return errors.New("snapshot changed while copying; will retry")
	}
	return nil
}

// removeDB deletes a database file and its WAL/shared-memory companions.
func removeDB(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm", path + "-journal"} {
		if err := removeIfExists(p); err != nil {
			return err
		}
	}
	return nil
}
