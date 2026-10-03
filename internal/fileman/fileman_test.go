package fileman

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevealMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.fit")
	err := Reveal(missing)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Reveal(missing) err = %v, want 'does not exist'", err)
	}
	if err := OpenDir(missing); err == nil {
		t.Fatal("OpenDir(missing) should fail")
	}
	if err := Reveal(""); err == nil {
		t.Fatal("Reveal(\"\") should fail")
	}
}

func TestResolveReturnsAbsolute(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.fit")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, info, err := resolve(f)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(abs) || info.IsDir() {
		t.Fatalf("resolve = %q dir=%v", abs, info.IsDir())
	}
}

func TestRunFirstEmpty(t *testing.T) {
	if err := runFirst(nil); err == nil {
		t.Fatal("runFirst(nil) should fail")
	}
}

func TestCommandsNonEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x y", "a.fit")
	for _, cmds := range [][]command{revealCommands(p), openDirCommands(filepath.Dir(p))} {
		if len(cmds) == 0 {
			t.Fatal("no commands built")
		}
		for _, c := range cmds {
			if len(c.args) == 0 {
				t.Fatal("command with no args")
			}
		}
	}
}
