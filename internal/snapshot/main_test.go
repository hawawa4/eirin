package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

// TestMain redirects any store.NewStore() call away from the real user config
// directory for the duration of the test binary.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "eirin-snapshot-test-*")
	if err != nil {
		panic("TestMain: could not create temp dir: " + err.Error())
	}
	if err := os.Setenv(store.EnvDBPath, filepath.Join(tmp, "prefs.db")); err != nil {
		panic("TestMain: could not set env var: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// must fatals if err is non-nil. Use for test setup calls that must succeed.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// newTestStore creates a fresh Store backed by a real SQLite file in a temp dir.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.NewStoreAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("newTestStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// desktopStore returns a store whose library root is root, with one stacked
// frame and one light frame under it.
func desktopStore(t *testing.T, root string) *store.Store {
	t.Helper()
	s := newTestStore(t)
	must(t, s.Set(store.KeyRootFolder, root))
	must(t, s.UpsertFrame(filepath.Join(root, "M31", "stack.fits"), store.Frame{Object: "M31", FrameType: store.FrameTypeStacked}))
	must(t, s.UpsertFrame(filepath.Join(root, "M31_sub", "light.fits"), store.Frame{Object: "M31", FrameType: store.FrameTypeLight}))
	return s
}
