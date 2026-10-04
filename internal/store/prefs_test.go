package store

import "testing"

func TestDefaultPrefs(t *testing.T) {
	p := DefaultPrefs()
	if p.StretchLevel != 2 {
		t.Errorf("StretchLevel = %d, want 2", p.StretchLevel)
	}
	if !p.StretchEnabled {
		t.Error("StretchEnabled should be true by default")
	}
	if !p.AdvancedCollapsed {
		t.Error("AdvancedCollapsed should be true by default")
	}
	if p.BasicCollapsed {
		t.Error("BasicCollapsed should be false by default")
	}
}

func TestSetAndLoad(t *testing.T) {
	s := newTestStore(t)

	// Fresh store returns defaults.
	p := s.Load()
	if p.StretchLevel != 2 {
		t.Errorf("default StretchLevel = %d, want 2", p.StretchLevel)
	}

	// Set values and reload.
	if err := s.Set(KeyRootFolder, "/nas/data"); err != nil {
		t.Fatalf("Set RootFolder: %v", err)
	}
	if err := s.Set(KeyStretchLevel, "3"); err != nil {
		t.Fatalf("Set StretchLevel: %v", err)
	}
	p = s.Load()
	if p.RootFolder != "/nas/data" {
		t.Errorf("RootFolder = %q, want /nas/data", p.RootFolder)
	}
	if p.StretchLevel != 3 {
		t.Errorf("StretchLevel = %d, want 3", p.StretchLevel)
	}

	// Overwrite an existing key.
	if err := s.Set(KeyRootFolder, "/nas/updated"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if p := s.Load(); p.RootFolder != "/nas/updated" {
		t.Errorf("updated RootFolder = %q, want /nas/updated", p.RootFolder)
	}
}

func TestLoadBoolPrefs(t *testing.T) {
	s := newTestStore(t)

	s.Set(KeyBasicCollapsed, "true")
	s.Set(KeyStretchEnabled, "false")
	p := s.Load()

	if !p.BasicCollapsed {
		t.Error("BasicCollapsed should be true after Set")
	}
	if p.StretchEnabled {
		t.Error("StretchEnabled should be false after Set")
	}
}

func TestLoadInvalidStretchLevel(t *testing.T) {
	s := newTestStore(t)
	s.Set(KeyStretchLevel, "notanumber")
	p := s.Load()
	// Falls back to default when value cannot be parsed.
	if p.StretchLevel != DefaultPrefs().StretchLevel {
		t.Errorf("StretchLevel with invalid stored value = %d, want default %d",
			p.StretchLevel, DefaultPrefs().StretchLevel)
	}
}

func TestLoadAllKeys(t *testing.T) {
	s := newTestStore(t)

	s.Set(KeySirilPath, "/usr/bin/siril")
	s.Set(KeyProjectsFolder, "/projects")
	s.Set(KeyTheme, "dark")
	s.Set(KeyColumnConfig, `["col1"]`)
	s.Set(KeyLibraryColumnConfig, `["col2"]`)

	p := s.Load()
	if p.SirilPath != "/usr/bin/siril" {
		t.Errorf("SirilPath = %q, want /usr/bin/siril", p.SirilPath)
	}
	if p.ProjectsFolder != "/projects" {
		t.Errorf("ProjectsFolder = %q, want /projects", p.ProjectsFolder)
	}
	if p.Theme != "dark" {
		t.Errorf("Theme = %q, want dark", p.Theme)
	}
	if p.ColumnConfig != `["col1"]` {
		t.Errorf("ColumnConfig = %q", p.ColumnConfig)
	}
	if p.LibraryColumnConfig != `["col2"]` {
		t.Errorf("LibraryColumnConfig = %q", p.LibraryColumnConfig)
	}
}

func TestUIStatePref(t *testing.T) {
	s := newTestStore(t)
	if p := s.Load(); p.UIState != "" {
		t.Errorf("default UIState = %q, want empty", p.UIState)
	}
	blob := `{"view":"library","sidebar":{"collapsed":true}}`
	must(t, s.Set(KeyUIState, blob))
	if p := s.Load(); p.UIState != blob {
		t.Errorf("UIState = %q, want %q", p.UIState, blob)
	}
}

func TestGetRawPref(t *testing.T) {
	s := newTestStore(t)
	if _, ok := s.Get("custom_key"); ok {
		t.Error("Get on a missing key reported ok")
	}
	must(t, s.Set("custom_key", "v"))
	if v, ok := s.Get("custom_key"); !ok || v != "v" {
		t.Errorf("Get = %q, %v, want \"v\", true", v, ok)
	}
}

func TestSnapshotEnabledPref(t *testing.T) {
	s := newTestStore(t)
	if s.Load().SnapshotEnabled {
		t.Error("SnapshotEnabled defaults to true, want false")
	}
	must(t, s.Set(KeySnapshotEnabled, "true"))
	if !s.Load().SnapshotEnabled {
		t.Error("SnapshotEnabled = false after setting it")
	}
}
