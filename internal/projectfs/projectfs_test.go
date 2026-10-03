package projectfs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSubdirFor(t *testing.T) {
	tests := []struct {
		frameType string
		want      string
		ok        bool
	}{
		{store.FrameTypeLight, DirLights, true},
		{store.FrameTypeDark, DirDarks, true},
		{store.FrameTypeFlat, DirFlats, true},
		{store.FrameTypeBias, DirBiases, true},
		{store.FrameTypeStacked, "", false},
		{store.FrameTypeProcessed, "", false},
		{store.FrameTypeImage, "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := SubdirFor(tt.frameType)
		if got != tt.want || ok != tt.ok {
			t.Errorf("SubdirFor(%q) = %q,%v want %q,%v", tt.frameType, got, ok, tt.want, tt.ok)
		}
		if tt.ok && FrameTypeForSubdir(got) != tt.frameType {
			t.Errorf("FrameTypeForSubdir(%q) != %q", got, tt.frameType)
		}
	}
	if IsFrameSubdir("process") {
		t.Error("process/ is not a frame subfolder")
	}
}

func TestCreate(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "projects", "M31_Ha")
	if err := Create(folder); err != nil {
		t.Fatal(err)
	}
	for _, d := range Subdirs {
		if info, err := os.Stat(filepath.Join(folder, d)); err != nil || !info.IsDir() {
			t.Errorf("%s/ not created", d)
		}
	}
	err := Create(folder)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second Create err = %v, want 'already exists'", err)
	}
}

func TestAddFrameSymlinkSameSourceNoop(t *testing.T) {
	nas, proj := t.TempDir(), t.TempDir()
	src := filepath.Join(nas, "Light_001.fit")
	writeFile(t, src, "a")

	r1, err := AddFrame(proj, DirLights, src, ModeSymlink)
	if err != nil || r1.Existed {
		t.Fatalf("first add: %+v %v", r1, err)
	}
	r2, err := AddFrame(proj, DirLights, src, ModeSymlink)
	if err != nil || !r2.Existed || r2.Path != r1.Path {
		t.Fatalf("second add: %+v %v", r2, err)
	}
	entries, _ := os.ReadDir(filepath.Join(proj, DirLights))
	if len(entries) != 1 {
		t.Errorf("entries = %d, want 1", len(entries))
	}
}

func TestAddFrameDifferentSourceGetsSuffix(t *testing.T) {
	nas, proj := t.TempDir(), t.TempDir()
	a := filepath.Join(nas, "night1", "Light_001.fit")
	b := filepath.Join(nas, "night2", "Light_001.fit")
	c := filepath.Join(nas, "night3", "Light_001.fit")
	writeFile(t, a, "a")
	writeFile(t, b, "b")
	writeFile(t, c, "c")

	ra, _ := AddFrame(proj, DirLights, a, ModeSymlink)
	rb, err := AddFrame(proj, DirLights, b, ModeSymlink)
	if err != nil {
		t.Fatal(err)
	}
	rc, _ := AddFrame(proj, DirLights, c, ModeSymlink)
	if filepath.Base(rb.Path) != "Light_001_2.fit" || filepath.Base(rc.Path) != "Light_001_3.fit" {
		t.Errorf("suffixed names = %s, %s", filepath.Base(rb.Path), filepath.Base(rc.Path))
	}
	// Original must still point to a.
	if got, _ := os.Readlink(ra.Path); got != a {
		t.Errorf("original link replaced: -> %s", got)
	}
	// Re-adding b finds its suffixed entry and is a no-op.
	if r, _ := AddFrame(proj, DirLights, b, ModeSymlink); !r.Existed || r.Path != rb.Path {
		t.Errorf("re-add b = %+v", r)
	}
}

func TestAddFrameCopyMode(t *testing.T) {
	nas, proj := t.TempDir(), t.TempDir()
	a := filepath.Join(nas, "x", "Dark_001.fit")
	b := filepath.Join(nas, "y", "Dark_001.fit")
	writeFile(t, a, "same")
	writeFile(t, b, "diff")

	r1, err := AddFrame(proj, DirDarks, a, ModeCopy)
	if err != nil {
		t.Fatal(err)
	}
	if lst, _ := os.Lstat(r1.Path); !lst.Mode().IsRegular() {
		t.Error("copy mode should create a regular file")
	}
	if r, _ := AddFrame(proj, DirDarks, a, ModeCopy); !r.Existed {
		t.Error("identical copy should be a no-op")
	}
	if r, _ := AddFrame(proj, DirDarks, b, ModeCopy); r.Existed || filepath.Base(r.Path) != "Dark_001_2.fit" {
		t.Errorf("different content = %+v", r)
	}
}

func TestAddFrameErrors(t *testing.T) {
	proj := t.TempDir()
	if _, err := AddFrame(proj, DirLights, "/nonexistent/x.fit", ModeSymlink); err == nil {
		t.Error("missing source should fail")
	}
	src := filepath.Join(t.TempDir(), "a.fit")
	writeFile(t, src, "a")
	if _, err := AddFrame(proj, DirLights, src, "hardlink"); err == nil {
		t.Error("bad mode should fail")
	}
}

func TestAddFrameReplacesDanglingLink(t *testing.T) {
	nas, proj := t.TempDir(), t.TempDir()
	src := filepath.Join(nas, "Light_001.fit")
	writeFile(t, src, "a")
	dangling := filepath.Join(proj, DirLights, "Light_001.fit")
	_ = os.MkdirAll(filepath.Dir(dangling), 0o755)
	if err := os.Symlink(filepath.Join(nas, "gone.fit"), dangling); err != nil {
		t.Fatal(err)
	}
	r, err := AddFrame(proj, DirLights, src, ModeSymlink)
	if err != nil || r.Path != dangling {
		t.Fatalf("got %+v %v", r, err)
	}
	if got, _ := os.Readlink(dangling); got != src {
		t.Errorf("link -> %s, want %s", got, src)
	}
}

func TestListFramesAllSubdirsAndLegacy(t *testing.T) {
	nas, proj := t.TempDir(), t.TempDir()
	light := filepath.Join(nas, "Light_001.fit")
	legacyDark := filepath.Join(nas, "Dark_001.fit")
	flat := filepath.Join(nas, "Flat_001.fit")
	for _, p := range []string{light, legacyDark, flat} {
		writeFile(t, p, filepath.Base(p))
	}
	// Legacy layout: dark symlinked into lights/.
	if _, err := AddFrame(proj, DirLights, light, ModeSymlink); err != nil {
		t.Fatal(err)
	}
	if _, err := AddFrame(proj, DirLights, legacyDark, ModeSymlink); err != nil {
		t.Fatal(err)
	}
	if _, err := AddFrame(proj, DirFlats, flat, ModeSymlink); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(proj, "result.fit"), "output") // root output: ignored

	entries, err := ListFrames(proj)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Subdir+"/"+e.Name+"->"+filepath.Base(e.SourcePath))
	}
	sort.Strings(got)
	want := []string{"flats/Flat_001.fit->Flat_001.fit", "lights/Dark_001.fit->Dark_001.fit", "lights/Light_001.fit->Light_001.fit"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("entries = %v, want %v", got, want)
	}
	for _, e := range entries {
		real, _ := filepath.EvalSymlinks(filepath.Join(nas, e.Name))
		if e.SourcePath != real {
			t.Errorf("%s: SourcePath = %s, want %s", e.Name, e.SourcePath, real)
		}
	}
}

func TestRemoveFrames(t *testing.T) {
	nas, proj := t.TempDir(), t.TempDir()
	a := filepath.Join(nas, "n1", "Light_001.fit")
	b := filepath.Join(nas, "n2", "Light_001.fit")
	d := filepath.Join(nas, "Dark_001.fit")
	writeFile(t, a, "a")
	writeFile(t, b, "b")
	writeFile(t, d, "d")
	_, _ = AddFrame(proj, DirLights, a, ModeSymlink)
	rb, _ := AddFrame(proj, DirLights, b, ModeSymlink) // Light_001_2.fit
	_, _ = AddFrame(proj, DirDarks, d, ModeSymlink)

	failed, err := RemoveFrames(proj, []string{b, d})
	if err != nil || failed != 0 {
		t.Fatalf("RemoveFrames: failed=%d err=%v", failed, err)
	}
	if _, err := os.Lstat(rb.Path); err == nil {
		t.Error("suffixed entry for b not removed")
	}
	if _, err := os.Lstat(filepath.Join(proj, DirLights, "Light_001.fit")); err != nil {
		t.Error("entry for a (same basename, different source) was removed")
	}
	if _, err := os.Lstat(filepath.Join(proj, DirDarks, "Dark_001.fit")); err == nil {
		t.Error("dark entry not removed")
	}
	for _, p := range []string{a, b, d} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("NAS file %s was touched", p)
		}
	}
}

func TestRemoveFramesCopyByLocalPath(t *testing.T) {
	nas, proj := t.TempDir(), t.TempDir()
	src := filepath.Join(nas, "Bias_001.fit")
	writeFile(t, src, "b")
	r, _ := AddFrame(proj, DirBiases, src, ModeCopy)
	if failed, err := RemoveFrames(proj, []string{r.Path}); err != nil || failed != 0 {
		t.Fatal(err)
	}
	if _, err := os.Lstat(r.Path); err == nil {
		t.Error("copied entry not removed")
	}
	if _, err := os.Stat(src); err != nil {
		t.Error("NAS source removed")
	}
}

func TestCopyNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, src, "new")
	writeFile(t, dst, "old")
	if err := CopyNoOverwrite(src, dst); err == nil {
		t.Fatal("expected error for existing destination")
	}
	if got, _ := os.ReadFile(dst); string(got) != "old" {
		t.Error("destination overwritten")
	}
}
