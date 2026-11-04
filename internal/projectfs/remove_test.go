package projectfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveDeletesFolderButNotLinkTargets(t *testing.T) {
	nas := t.TempDir()
	src := filepath.Join(nas, "light_001.fit")
	writeFile(t, src, "pixels")

	root := t.TempDir()
	folder := filepath.Join(root, "M31")
	if err := Create(folder); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, filepath.Join(folder, DirLights, "light_001.fit")); err != nil {
		t.Fatal(err)
	}
	// A symlinked directory inside the project must not be descended into.
	if err := os.Symlink(nas, filepath.Join(folder, "nas-link")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(folder, "result.fit"), "stack")

	if err := Remove(root, folder); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Lstat(folder); !os.IsNotExist(err) {
		t.Errorf("project folder still exists (err=%v)", err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("NAS frame behind the symlink was touched: %v", err)
	}
}

func TestRemoveMissingFolderIsNoop(t *testing.T) {
	root := t.TempDir()
	if err := Remove(root, filepath.Join(root, "gone")); err != nil {
		t.Errorf("Remove(missing): %v", err)
	}
}

func TestRemoveRefusesOutsideProjectsRoot(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	writeFile(t, filepath.Join(other, "keep.txt"), "x")

	cases := map[string]string{
		"root itself":     root,
		"sibling folder":  other,
		"parent escape":   filepath.Join(root, "..", filepath.Base(other)),
		"filesystem root": string(filepath.Separator),
	}
	for name, folder := range cases {
		if err := Remove(root, folder); err == nil {
			t.Errorf("%s: expected an error for %q", name, folder)
		}
	}
	if err := Remove("", filepath.Join(root, "x")); err == nil {
		t.Error("empty projects root: expected an error")
	}
	if _, err := os.Stat(filepath.Join(other, "keep.txt")); err != nil {
		t.Errorf("file outside the projects root was touched: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Errorf("projects root was touched: %v", err)
	}
}

func TestRemoveRefusesFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "notes.txt")
	writeFile(t, f, "x")
	if err := Remove(root, f); err == nil {
		t.Error("expected an error for a file")
	}
}
