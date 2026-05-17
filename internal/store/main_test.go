package store

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain redirects any NewStore() call away from the real user config
// directory for the duration of the test binary.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "eirin-prefs-test-*")
	if err != nil {
		panic("TestMain: could not create temp dir: " + err.Error())
	}
	if err := os.Setenv(EnvDBPath, filepath.Join(tmp, "prefs.db")); err != nil {
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

// newTestStore creates a fresh Store backed by a real SQLite file in the
// system temp directory. The file and store are cleaned up after the test.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStoreAt(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("newTestStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
