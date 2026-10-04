package snapshot

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TaruDesigns/eirin/internal/store"
)

// testLoader returns a loader serving the library at serverRoot, and a pointer
// to the store it currently serves (updated on every swap).
func testLoader(t *testing.T, serverRoot string) (*Loader, **store.Store) {
	t.Helper()
	var current *store.Store
	local := filepath.Join(t.TempDir(), "viewer", "library.db")
	must(t, os.MkdirAll(filepath.Dir(local), 0o755))
	l := NewLoader(serverRoot, local, func(st *store.Store) *store.Store {
		old := current
		current = st
		return old
	})
	l.interval = time.Hour
	st, err := l.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	current = st
	t.Cleanup(func() {
		if current != nil {
			_ = current.Close()
		}
	})
	return l, &current
}

func TestLoaderOpenWithoutSnapshot(t *testing.T) {
	root := t.TempDir()
	l, cur := testLoader(t, root)

	if got, _ := (*cur).Get(store.KeyRootFolder); got != root {
		t.Errorf("empty viewer root = %q, want %q", got, root)
	}
	if l.Check() {
		t.Error("Check swapped without a snapshot")
	}
	if st := l.Status(); st.Loaded || st.LastError != "" || st.Root != root {
		t.Errorf("Status() = %+v", st)
	}
}

func TestLoaderLoadsRewritesAndPrunes(t *testing.T) {
	desktopRoot := "/media/desktop/Astro"
	serverRoot := t.TempDir()
	// The snapshot lives on the "NAS", i.e. under the server's root.
	must(t, Publish(desktopStore(t, desktopRoot), serverRoot))

	l, cur := testLoader(t, serverRoot)
	if !l.Check() {
		t.Fatalf("Check didn't load: %+v", l.Status())
	}

	st := *cur
	if ok, _ := st.HasFrame(filepath.Join(serverRoot, "M31", "stack.fits")); !ok {
		t.Error("stacked frame missing or not rewritten to the server root")
	}
	if n, _ := st.FrameCount(); n != 1 {
		t.Errorf("viewer has %d frames, want 1 (lights pruned)", n)
	}
	if err := st.Set(store.KeyTheme, "red"); err == nil {
		t.Error("viewer database is writable")
	}
	s := l.Status()
	if !s.Loaded || s.SourceRoot != desktopRoot || s.Frames != 1 || s.PublishedAt.IsZero() || s.LoadedAt.IsZero() {
		t.Errorf("Status() = %+v", s)
	}
	if l.Check() {
		t.Error("reloaded an unchanged snapshot")
	}
}

func TestLoaderPicksUpRepublish(t *testing.T) {
	root := t.TempDir()
	desk := desktopStore(t, "/desktop")
	must(t, Publish(desk, root))
	l, cur := testLoader(t, root)
	l.Check()
	first := *cur

	must(t, desk.UpsertFrame("/desktop/M42/stack.fits", store.Frame{FrameType: store.FrameTypeStacked}))
	must(t, Publish(desk, root))
	// Make sure the mtime differs even on coarse-grained filesystems.
	future := time.Now().Add(2 * time.Second)
	must(t, os.Chtimes(Path(root), future, future))

	if !l.Check() {
		t.Fatal("republished snapshot not picked up")
	}
	if *cur == first {
		t.Error("store not swapped")
	}
	if n, _ := (*cur).FrameCount(); n != 2 {
		t.Errorf("viewer has %d frames, want 2", n)
	}
}

func TestLoaderRejectsNewerSchema(t *testing.T) {
	root := t.TempDir()
	desk := desktopStore(t, "/desktop")
	must(t, Publish(desk, root))
	l, cur := testLoader(t, root)
	l.Check()
	before := *cur

	// Simulate a snapshot from a newer Eirin: one more applied migration.
	must(t, Publish(desk, root))
	db, err := sql.Open("sqlite", Path(root))
	must(t, err)
	_, err = db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, store.LatestSchemaVersion()+1)
	must(t, err)
	must(t, db.Close())
	future := time.Now().Add(2 * time.Second)
	must(t, os.Chtimes(Path(root), future, future))

	if l.Check() {
		t.Fatal("loaded a snapshot from a newer schema")
	}
	if *cur != before {
		t.Error("store swapped despite the error")
	}
	if l.Status().LastError == "" {
		t.Error("error not reported in Status")
	}
	if l.Check() {
		t.Error("retried the same broken snapshot")
	}
}

func TestLoaderReopensLocalCopyAfterRestart(t *testing.T) {
	root := t.TempDir()
	must(t, Publish(desktopStore(t, "/desktop"), root))
	local := filepath.Join(t.TempDir(), "library.db")
	swap := func(st *store.Store) *store.Store { return nil }

	l1 := NewLoader(root, local, swap)
	st1, err := l1.Open()
	must(t, err)
	l1.Check()
	_ = st1.Close()

	l2 := NewLoader(root, local, swap)
	st2, err := l2.Open()
	must(t, err)
	defer st2.Close()
	if n, _ := st2.FrameCount(); n != 1 {
		t.Errorf("reopened copy has %d frames, want 1", n)
	}
	if !l2.Status().Loaded {
		t.Error("reopened copy not reported as loaded")
	}
	if l2.Check() {
		t.Error("reloaded the snapshot it already had after a restart")
	}
}

func TestLoaderDiscardsCopyForAnotherRoot(t *testing.T) {
	rootA, rootB := t.TempDir(), t.TempDir()
	must(t, Publish(desktopStore(t, "/desktop"), rootA))
	local := filepath.Join(t.TempDir(), "library.db")
	swap := func(st *store.Store) *store.Store { return nil }

	l1 := NewLoader(rootA, local, swap)
	st1, err := l1.Open()
	must(t, err)
	l1.Check()
	_ = st1.Close()

	l2 := NewLoader(rootB, local, swap)
	st2, err := l2.Open()
	must(t, err)
	defer st2.Close()
	if n, _ := st2.FrameCount(); n != 0 {
		t.Errorf("copy for another root kept %d frames", n)
	}
}
