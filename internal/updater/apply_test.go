package updater

import (
	"archive/zip"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func TestExtractBinary(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	writeZip(t, zipPath, map[string]string{"eirin": "new-binary", "README": "hi"})

	dest := filepath.Join(dir, "out")
	if err := extractBinary(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "new-binary" {
		t.Errorf("got %q", got)
	}
	if st, _ := os.Stat(dest); st.Mode().Perm()&0o100 == 0 {
		t.Error("extracted binary is not executable")
	}

	writeZip(t, zipPath, map[string]string{"README": "hi"})
	if err := extractBinary(zipPath, dest); err == nil {
		t.Error("expected an error when the archive has no executable")
	}
}

func TestReplaceFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "eirin")
	if err := os.WriteFile(target, []byte("old"), 0o750); err != nil {
		t.Fatal(err)
	}
	staged, err := tempBeside(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staged, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(staged, target); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Errorf("target not replaced: %q", got)
	}
	if st, _ := os.Stat(target); st.Mode().Perm()&0o100 == 0 {
		t.Error("replacement lost its executable bit")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temp file left behind: %v", entries)
	}
}

func TestTempBesideUnwritableDir(t *testing.T) {
	if _, err := tempBeside("/nonexistent-dir/eirin"); err == nil {
		t.Error("expected an error for a missing directory")
	}
}

func TestDebInstallCmd(t *testing.T) {
	cmd := debInstallCmd(t.Context(), "/tmp/x/eirin.deb")
	want := []string{"pkexec", "env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y", "/tmp/x/eirin.deb"}
	if !slices.Equal(cmd.Args, want) {
		t.Errorf("args = %v", cmd.Args)
	}
}

func TestTail(t *testing.T) {
	if got := tail("  short ", 10); got != "short" {
		t.Errorf("got %q", got)
	}
	if got := tail("0123456789", 4); got != "…6789" {
		t.Errorf("got %q", got)
	}
}
