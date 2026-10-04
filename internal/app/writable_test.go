package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func TestRequireWritable(t *testing.T) {
	err := newTestApp(t).requireWritable()
	if desktopModeEnabled && err != nil {
		t.Errorf("desktop build: requireWritable() = %v, want nil", err)
	}
	if !desktopModeEnabled && !errors.Is(err, errReadOnly) {
		t.Errorf("server build: requireWritable() = %v, want errReadOnly", err)
	}
}

// TestWriteMethodsReadOnlyInServer checks that, in server builds, every write
// method refuses and leaves the database and the files untouched.
func TestWriteMethodsReadOnlyInServer(t *testing.T) {
	if desktopModeEnabled {
		t.Skip("server builds only")
	}
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "stack.fits")
	must(t, os.WriteFile(path, []byte("x"), 0o644))
	must(t, a.store().UpsertFrame(path, store.Frame{FrameType: store.FrameTypeStacked}))

	calls := map[string]func() error{
		"RejectFile":           func() error { return a.RejectFile(path) },
		"UnrejectFile":         func() error { return a.UnrejectFile(path) },
		"BatchRejectFiles":     func() error { return a.BatchRejectFiles([]string{path}) },
		"BatchUnrejectFiles":   func() error { return a.BatchUnrejectFiles([]string{path}) },
		"HardDeleteFile":       func() error { return a.HardDeleteFile(path) },
		"BatchHardDeleteFiles": func() error { return a.BatchHardDeleteFiles([]string{path}) },
		"RenameFrame":          func() error { _, err := a.RenameFrame(path, "x.fits"); return err },
		"UpdateFrameMeta":      func() error { return a.UpdateFrameMeta(path, store.FrameMeta{}) },
		"SetFrameType":         func() error { return a.SetFrameType(path, store.FrameTypeLight) },
		"SetProjectsFolder":    func() error { return a.SetProjectsFolder(dir) },
		"CreateProject":        func() error { _, err := a.CreateProject("p", ""); return err },
		"DeleteProject":        func() error { return a.DeleteProject(1) },
		"AddFramesToProject":   func() error { return a.AddFramesToProject(dir, []string{path}, "copy") },
		"AddFramesToProjectDetailed": func() error {
			_, err := a.AddFramesToProjectDetailed(dir, []string{path}, "copy")
			return err
		},
		"ImportOutputFiles":       func() error { _, err := a.ImportOutputFiles([]string{path}, dir); return err },
		"RemoveFramesFromProject": func() error { return a.RemoveFramesFromProject(dir, []string{path}) },
		"SetSirilPath":            func() error { return a.SetSirilPath("/usr/bin/siril") },
		"BackupDatabase":          func() error { _, err := a.BackupDatabase(); return err },
		"PublishSnapshot":         a.PublishSnapshot,
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, errReadOnly) {
			t.Errorf("%s() = %v, want errReadOnly", name, err)
		}
	}

	a.SetPref(store.KeyTheme, "red")
	if a.LoadPrefs().Theme == "red" {
		t.Error("SetPref wrote to the database")
	}
	a.BuildIndex(dir, true)
	if a.indexer.gen != 0 {
		t.Error("BuildIndex started indexing")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file touched: %v", err)
	}
	frames, err := a.store().GetFrames([]string{path})
	must(t, err)
	if f := frames[path]; f.Rejected || f.FrameType != store.FrameTypeStacked {
		t.Errorf("frame modified: %+v", f)
	}
}
