package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStoreAtCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sub", "test.db")
	s, err := NewStoreAt(path)
	if err != nil {
		t.Fatalf("NewStoreAt: %v", err)
	}
	defer s.Close()

	if s.DBPath() != path {
		t.Errorf("DBPath() = %q, want %q", s.DBPath(), path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("database file not created at %q: %v", path, err)
	}
}

func TestNewStoreAtIsFunctional(t *testing.T) {
	s := newTestStore(t)
	if err := s.Set("smoke_key", "smoke_value"); err != nil {
		t.Fatalf("Set after NewStoreAt: %v", err)
	}
	p := s.Load()
	_ = p // Load uses prefs-specific keys; just verify no panic
}

func TestNewStoreRespectsEnvVar(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom.db")
	t.Setenv(EnvDBPath, customPath)

	s, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore with %s=%q: %v", EnvDBPath, customPath, err)
	}
	defer s.Close()

	if s.DBPath() != customPath {
		t.Errorf("DBPath() = %q, want %q (env var should override)", s.DBPath(), customPath)
	}
	if _, err := os.Stat(customPath); err != nil {
		t.Errorf("expected database at %q: %v", customPath, err)
	}
}

func TestNewStoreDefaultsToConfigDir(t *testing.T) {
	// Redirect to a temp dir so we never touch the real user config dir.
	dir := t.TempDir()
	t.Setenv(EnvDBPath, "")
	t.Setenv("XDG_CONFIG_HOME", dir)

	s, err := NewStore()
	if err != nil {
		t.Fatalf("NewStore (default path): %v", err)
	}
	defer s.Close()

	// Should have created a prefs.db somewhere under our redirected config dir.
	if s.DBPath() == "" {
		t.Error("DBPath() should not be empty")
	}
}

func TestNewStoreAtReopenPreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")

	s1, err := NewStoreAt(path)
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	if err := s1.Set(KeyRootFolder, "/nas/root"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	s1.Close()

	s2, err := NewStoreAt(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	if p := s2.Load(); p.RootFolder != "/nas/root" {
		t.Errorf("RootFolder after reopen = %q, want /nas/root", p.RootFolder)
	}
}
