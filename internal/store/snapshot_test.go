package store

import (
	"os"
	"path/filepath"
	"testing"
)

// vacuumCopy writes a snapshot of s to a temp file and opens it as a writable store.
func vacuumCopy(t *testing.T, s *Store) *Store {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "copy.db")
	must(t, s.VacuumInto(dst))
	c, err := NewStoreAt(dst)
	if err != nil {
		t.Fatalf("open copy: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestLatestSchemaVersionMatchesFreshStore(t *testing.T) {
	s := newTestStore(t)
	v, err := s.SchemaVersion()
	must(t, err)
	if v != LatestSchemaVersion() {
		t.Errorf("SchemaVersion() = %d, want %d", v, LatestSchemaVersion())
	}
}

func TestVacuumIntoIncludesUncheckpointedRows(t *testing.T) {
	s := newTestStore(t)
	// Fresh writes in WAL mode live in the -wal file until a checkpoint.
	must(t, s.UpsertFrame("/nas/M31/a.fits", Frame{Object: "M31", FrameType: FrameTypeStacked}))
	must(t, s.Set(KeyRootFolder, "/nas"))

	c := vacuumCopy(t, s)
	got, err := c.GetFrames([]string{"/nas/M31/a.fits"})
	must(t, err)
	if got["/nas/M31/a.fits"].Object != "M31" {
		t.Errorf("copy is missing the frame: %+v", got)
	}
	if c.Load().RootFolder != "/nas" {
		t.Errorf("copy root folder = %q, want /nas", c.Load().RootFolder)
	}
}

func TestVacuumIntoReplacesExistingFile(t *testing.T) {
	s := newTestStore(t)
	dst := filepath.Join(t.TempDir(), "copy.db")
	must(t, os.WriteFile(dst, []byte("stale"), 0o644))
	if err := s.VacuumInto(dst); err != nil {
		t.Fatalf("VacuumInto over an existing file: %v", err)
	}
}

func TestChangeCountTracksWrites(t *testing.T) {
	s := newTestStore(t)
	before, err := s.ChangeCount()
	must(t, err)
	must(t, s.Set(KeyTheme, "red"))
	after, err := s.ChangeCount()
	must(t, err)
	if after == before {
		t.Errorf("ChangeCount unchanged after a write (%d)", after)
	}
	again, err := s.ChangeCount()
	must(t, err)
	if again != after {
		t.Errorf("ChangeCount changed without a write: %d -> %d", after, again)
	}
}

func TestRewriteRoot(t *testing.T) {
	s := newTestStore(t)
	old := "/media/Astro_Ω" // underscore is a LIKE wildcard; Ω is multi-byte
	must(t, s.UpsertFrame(old+"/M31/a.fits", Frame{FrameType: FrameTypeStacked}))
	must(t, s.UpsertFrame(old+"/M 31_sub/b.fits", Frame{FrameType: FrameTypeLight}))
	must(t, s.UpsertFrame("/media/AstroXΩ/c.fits", Frame{FrameType: FrameTypeStacked})) // not under root
	must(t, s.UpsertFrame("/elsewhere/d.fits", Frame{FrameType: FrameTypeStacked}))

	must(t, s.RewriteRoot(old+"/", "/library"))

	for _, p := range []string{"/library/M31/a.fits", "/library/M 31_sub/b.fits"} {
		ok, err := s.HasFrame(p)
		must(t, err)
		if !ok {
			t.Errorf("missing rewritten frame %q", p)
		}
	}
	n, err := s.FrameCount()
	must(t, err)
	if n != 2 {
		t.Errorf("FrameCount() = %d, want 2 (frames outside the root dropped)", n)
	}
	if got := s.Load().RootFolder; got != "/library" {
		t.Errorf("root folder = %q, want /library", got)
	}
}

func TestRewriteRootSameRootKeepsPaths(t *testing.T) {
	s := newTestStore(t)
	must(t, s.UpsertFrame("/nas/a.fits", Frame{FrameType: FrameTypeStacked}))
	must(t, s.RewriteRoot("/nas", "/nas/"))
	ok, err := s.HasFrame("/nas/a.fits")
	must(t, err)
	if !ok {
		t.Error("frame lost when roots are equal")
	}
}

func TestPruneToFinals(t *testing.T) {
	s := newTestStore(t)
	keep := map[string]string{
		"/nas/stacked.fits":   FrameTypeStacked,
		"/nas/processed.fits": FrameTypeProcessed,
		"/nas/final.png":      FrameTypeImage,
	}
	drop := map[string]string{
		"/nas/light.fits": FrameTypeLight,
		"/nas/dark.fits":  FrameTypeDark,
		"/nas/flat.fits":  FrameTypeFlat,
		"/nas/bias.fits":  FrameTypeBias,
	}
	for p, ft := range keep {
		must(t, s.UpsertFrame(p, Frame{FrameType: ft}))
	}
	for p, ft := range drop {
		must(t, s.UpsertFrame(p, Frame{FrameType: ft}))
	}
	must(t, s.UpsertFrame("/nas/rejected.fits", Frame{FrameType: FrameTypeStacked}))
	must(t, s.RejectFrame("/nas/rejected.fits", ""))
	_, err := s.CreateProject("p", "", "/projects/p")
	must(t, err)
	must(t, s.Set(KeyRootFolder, "/nas"))
	must(t, s.Set(KeySirilPath, "/usr/bin/siril"))

	must(t, s.PruneToFinals())

	for p := range keep {
		if ok, _ := s.HasFrame(p); !ok {
			t.Errorf("final frame %q was pruned", p)
		}
	}
	for p := range drop {
		if ok, _ := s.HasFrame(p); ok {
			t.Errorf("non-final frame %q was kept", p)
		}
	}
	if ok, _ := s.HasFrame("/nas/rejected.fits"); ok {
		t.Error("rejected frame was kept")
	}
	projects, err := s.ListProjects()
	must(t, err)
	if len(projects) != 0 {
		t.Errorf("projects kept: %v", projects)
	}
	p := s.Load()
	if p.RootFolder != "/nas" || p.SirilPath != "" {
		t.Errorf("prefs after prune: root=%q siril=%q, want root kept and the rest dropped", p.RootFolder, p.SirilPath)
	}
}

func TestOpenReadOnly(t *testing.T) {
	s := newTestStore(t)
	must(t, s.UpsertFrame("/nas/a b/c.fits", Frame{FrameType: FrameTypeStacked}))
	c := vacuumCopy(t, s)
	must(t, c.Compact())
	path := c.DBPath()
	must(t, c.Close())

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer ro.Close()

	if ok, err := ro.HasFrame("/nas/a b/c.fits"); err != nil || !ok {
		t.Errorf("HasFrame on read-only store = %v, %v", ok, err)
	}
	if err := ro.Set(KeyTheme, "red"); err == nil {
		t.Error("write to a read-only store succeeded")
	}
	if _, err := os.Stat(path + "-wal"); err == nil {
		t.Error("read-only open created a -wal file")
	}
}

func TestOpenReadOnlyMissingFile(t *testing.T) {
	if _, err := OpenReadOnly(filepath.Join(t.TempDir(), "nope.db")); err == nil {
		t.Error("OpenReadOnly on a missing file succeeded")
	}
}
