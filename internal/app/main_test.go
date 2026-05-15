package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TaruDesigns/eirin/internal/prefs"
)

// TestMain redirects any NewStore() call that might reach production code
// during tests away from the real user config directory.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "eirin-app-test-*")
	if err != nil {
		panic("TestMain: could not create temp dir: " + err.Error())
	}
	if err := os.Setenv(prefs.EnvDBPath, filepath.Join(tmp, "prefs.db")); err != nil {
		panic("TestMain: could not set env var: " + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// newTestApp returns an App wired to a fresh SQLite store in the system temp
// directory. Both the store and its database file are cleaned up after the test.
func newTestApp(t *testing.T) *App {
	t.Helper()
	store, err := prefs.NewStoreAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("newTestApp: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &App{prefs: store}
}
