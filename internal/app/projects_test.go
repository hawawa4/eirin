package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My Project", "My_Project"},
		{"NGC 1234 Ha 300s", "NGC_1234_Ha_300s"},
		{"valid-name_here", "valid-name_here"},
		{"M42", "M42"},
		// Special characters are collapsed to a single underscore
		{"special!@#chars", "special_chars"},
		{"a  b", "a_b"}, // consecutive spaces → one underscore
		// Underscores at edges are trimmed
		{"!leading", "leading"},
		{"trailing!", "trailing"},
		// Empty / whitespace-only → fallback
		{"", "project"},
		{"   ", "project"},
		// Hyphens are preserved (they are in the allowed set)
		{"my-project-2024", "my-project-2024"},
		// Only invalid chars → fallback after trim
		{"!!!!", "project"},
	}

	for _, tt := range tests {
		got := sanitizeFolderName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFolderName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddFramesToProjectRoutesByFrameType(t *testing.T) {
	a := newTestApp(t)
	nas, proj := t.TempDir(), t.TempDir()
	frames := map[string]string{
		"sub1.fit":   store.FrameTypeLight,
		"d.fit":      store.FrameTypeDark,
		"f.fit":      store.FrameTypeFlat,
		"b.fit":      store.FrameTypeBias,
		"stack.fit":  store.FrameTypeStacked,
		"render.png": store.FrameTypeImage,
	}
	var paths []string
	for name, ft := range frames {
		p := filepath.Join(nas, name)
		writeTestFile(t, p, name)
		if err := a.store.UpsertFrame(p, store.Frame{FrameType: ft}); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	res, err := a.AddFramesToProjectDetailed(proj, paths, "symlink")
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 4 || len(res.Skipped) != 2 {
		t.Errorf("result = %+v, want 4 added / 2 skipped", res)
	}
	for name, sub := range map[string]string{"sub1.fit": "lights", "d.fit": "darks", "f.fit": "flats", "b.fit": "biases"} {
		if _, err := os.Lstat(filepath.Join(proj, sub, name)); err != nil {
			t.Errorf("%s not in %s/", name, sub)
		}
	}

	// Re-adding is a no-op; only-unsupported selection is an error.
	if err := a.AddFramesToProject(proj, paths, "symlink"); err != nil {
		t.Errorf("re-add: %v", err)
	}
	if err := a.AddFramesToProject(proj, []string{filepath.Join(nas, "stack.fit")}, "symlink"); err == nil {
		t.Error("adding only a stacked frame should fail")
	}
	if err := a.AddFramesToProject(proj, paths, "bogus"); err == nil {
		t.Error("bad mode should fail")
	}
}

func TestAddFramesToProjectUnindexedFallsBackToPath(t *testing.T) {
	a := newTestApp(t)
	nas, proj := t.TempDir(), t.TempDir()
	p := filepath.Join(nas, "Flat_001.fit") // not in DB → classified by name
	writeTestFile(t, p, "x")
	if err := a.AddFramesToProject(proj, []string{p}, "symlink"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(proj, "flats", "Flat_001.fit")); err != nil {
		t.Error("unindexed flat not routed to flats/")
	}
}

func TestGetProjectLibraryFramesLegacyAndTypes(t *testing.T) {
	a := newTestApp(t)
	nas, proj := t.TempDir(), t.TempDir()
	light := filepath.Join(nas, "l.fit")
	dark := filepath.Join(nas, "d.fit")
	unknown := filepath.Join(nas, "u.fit")
	for _, p := range []string{light, dark, unknown} {
		writeTestFile(t, p, filepath.Base(p))
	}
	_ = a.store.UpsertFrame(light, store.Frame{FrameType: store.FrameTypeLight})
	_ = a.store.UpsertFrame(dark, store.Frame{FrameType: store.FrameTypeDark})
	// Legacy layout: everything in lights/, plus an unindexed frame in flats/.
	_ = os.MkdirAll(filepath.Join(proj, "lights"), 0o755)
	_ = os.MkdirAll(filepath.Join(proj, "flats"), 0o755)
	_ = os.Symlink(light, filepath.Join(proj, "lights", "l.fit"))
	_ = os.Symlink(dark, filepath.Join(proj, "lights", "d.fit"))
	_ = os.Symlink(unknown, filepath.Join(proj, "flats", "u.fit"))

	got := map[string]string{}
	for _, f := range a.GetProjectLibraryFrames(proj) {
		got[f.FileName] = f.FrameType
	}
	want := map[string]string{"l.fit": "light", "d.fit": "dark", "u.fit": "flat"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s frameType = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}

	names, _ := a.GetProjectFrames(proj)
	if len(names) != 3 {
		t.Errorf("GetProjectFrames = %v", names)
	}
	if err := a.RemoveFramesFromProject(proj, []string{dark}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(proj, "lights", "d.fit")); err == nil {
		t.Error("legacy dark in lights/ not removed")
	}
}

func TestCreateProjectFolderCollision(t *testing.T) {
	a := newTestApp(t)
	pf := t.TempDir()
	if err := a.store.Set(store.KeyProjectsFolder, pf); err != nil {
		t.Fatal(err)
	}
	p, err := a.CreateProject("M31 Ha", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"lights", "darks", "flats", "biases"} {
		if _, err := os.Stat(filepath.Join(p.Folder, d)); err != nil {
			t.Errorf("%s/ missing", d)
		}
	}
	if _, err := a.CreateProject("M31_Ha", ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("colliding project err = %v", err)
	}
	if list, _ := a.ListProjects(); len(list) != 1 {
		t.Errorf("projects in DB = %d, want 1", len(list))
	}
}

func TestImportOutputFilesNoOverwrite(t *testing.T) {
	a := newTestApp(t)
	proj, dest := t.TempDir(), t.TempDir()
	out1 := filepath.Join(proj, "result.png")
	out2 := filepath.Join(proj, "other.png")
	writeTestFile(t, out1, "new")
	writeTestFile(t, out2, "new2")
	writeTestFile(t, filepath.Join(dest, "result.png"), "old")

	err := a.ImportOutputFiles([]string{out1, out2}, dest)
	if err == nil || !strings.Contains(err.Error(), "result.png") {
		t.Fatalf("err = %v, want conflict naming result.png", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "result.png")); string(got) != "old" {
		t.Error("existing NAS file overwritten")
	}
	if _, err := os.Stat(filepath.Join(dest, "other.png")); err == nil {
		t.Error("nothing should be copied when there is a conflict")
	}

	if err := a.ImportOutputFiles([]string{out2}, dest); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dest, "other.png")); string(got) != "new2" {
		t.Error("non-conflicting file not copied")
	}
}

func TestGetProjectOutputFilesSkipsFrameDirs(t *testing.T) {
	a := newTestApp(t)
	proj := t.TempDir()
	for _, d := range []string{"lights", "darks", "flats", "biases", "process"} {
		_ = os.MkdirAll(filepath.Join(proj, d), 0o755)
	}
	writeTestFile(t, filepath.Join(proj, "result.fit"), "x")
	files, err := a.GetProjectOutputFiles(proj)
	if err != nil || len(files) != 1 || files[0].Name != "result.fit" {
		t.Errorf("files = %+v err = %v", files, err)
	}
}
