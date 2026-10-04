//go:build !server

package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TaruDesigns/eirin/internal/importer"
	"github.com/TaruDesigns/eirin/internal/store"
)

func waitImportFinished(t *testing.T, a *App) ImportProgress {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.imports.mu.Lock()
		running := a.imports.running
		a.imports.mu.Unlock()
		if !running {
			return a.GetImportStatus()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("import did not finish")
	return ImportProgress{}
}

func TestStartImportEndToEnd(t *testing.T) {
	a := newTestApp(t)
	nas, src := t.TempDir(), t.TempDir()
	if err := a.store().Set(store.KeyRootFolder, nas); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(src, "M31", "a.png"), "frame a")
	writeTestFile(t, filepath.Join(src, "M31", "b.png"), "frame b")

	if got := a.GetImportStatus(); got.Phase != importer.PhaseIdle {
		t.Errorf("initial phase = %q, want idle", got.Phase)
	}
	if err := a.StartImport(src, []string{"png"}, true); err != nil {
		t.Fatal(err)
	}
	p := waitImportFinished(t, a)
	if p.Phase != importer.PhaseDone || p.Copied != 2 || p.Current != 2 || p.Total != 2 {
		t.Errorf("final progress = %+v", p)
	}
	for _, n := range []string{"a.png", "b.png"} {
		if _, err := os.Stat(filepath.Join(nas, "M31", n)); err != nil {
			t.Errorf("%s not copied", n)
		}
		if _, err := os.Stat(filepath.Join(src, "M31", n)); err == nil {
			t.Errorf("%s not deleted from source after verified copy", n)
		}
	}
}

func TestStartImportRejectsConcurrentRun(t *testing.T) {
	a := newTestApp(t)
	a.imports.running = true
	if err := a.StartImport(t.TempDir(), nil, false); err != errImportRunning {
		t.Errorf("err = %v, want errImportRunning", err)
	}
}

func TestStartImportFatalRootError(t *testing.T) {
	a := newTestApp(t)
	if err := a.store().Set(store.KeyRootFolder, filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal(err)
	}
	if err := a.StartImport(t.TempDir(), nil, false); err == nil {
		t.Fatal("expected error for inaccessible root")
	}
	p := a.GetImportStatus()
	if p.Phase != importer.PhaseError || p.Error == "" {
		t.Errorf("status = %+v", p)
	}
	if a.imports.running {
		t.Error("job slot not released after fatal error")
	}
}

func TestCancelImportIdleNoop(t *testing.T) {
	a := newTestApp(t)
	a.CancelImport() // must not panic
}
