package importer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeLib is an in-memory Library keyed by prefix hash.
type fakeLib struct {
	byHash   map[string][]string
	imported []Candidate
}

func newFakeLib() *fakeLib { return &fakeLib{byHash: map[string][]string{}} }

func (l *fakeLib) add(t *testing.T, path string) {
	t.Helper()
	h, err := HashFilePrefix(path)
	if err != nil {
		t.Fatal(err)
	}
	l.byHash[h] = append(l.byHash[h], path)
}
func (l *fakeLib) HasPrefixHash(h string) bool { return len(l.byHash[h]) > 0 }
func (l *fakeLib) PathsWithPrefixHash(h string) ([]string, error) {
	return l.byHash[h], nil
}
func (l *fakeLib) FileImported(c Candidate) { l.imported = append(l.imported, c) }

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func candidate(src, nas, name string) Candidate {
	return Candidate{
		SourcePath:   filepath.Join(src, name),
		RelativePath: name,
		DestPath:     filepath.Join(nas, "obj", name),
	}
}

// prefixCollision returns two different payloads that share the first 1 MiB.
func prefixCollision() (a, b []byte) {
	head := bytes.Repeat([]byte{0xAB}, hashPrefixBytes)
	a = append(append([]byte{}, head...), []byte("tail-A")...)
	b = append(append([]byte{}, head...), []byte("tail-B")...)
	return a, b
}

func TestRunPrefixCollisionKeepsSource(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	libData, srcData := prefixCollision()
	libPath := filepath.Join(nas, "obj", "existing.fit")
	writeFile(t, libPath, libData)
	writeFile(t, filepath.Join(src, "new.fit"), srcData)
	lib := newFakeLib()
	lib.add(t, libPath)

	c := candidate(src, nas, "new.fit")
	p := Run(context.Background(), []Candidate{c}, lib, Options{DeleteAfterCopy: true}, nil)

	if !exists(c.SourcePath) {
		t.Fatal("source was deleted although only the prefix matched")
	}
	if p.Skipped != 1 || p.Kept != 1 || p.Copied != 0 {
		t.Errorf("counts = skipped %d kept %d copied %d, want 1/1/0", p.Skipped, p.Kept, p.Copied)
	}
	if len(p.Notes) != 1 || !strings.Contains(p.Notes[0], noteNotVerified) {
		t.Errorf("notes = %v", p.Notes)
	}
	if p.Phase != PhaseDone || p.Current != 1 {
		t.Errorf("phase/current = %s/%d", p.Phase, p.Current)
	}
}

func TestRunIdenticalDuplicateDeleted(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	data := []byte("identical frame data")
	libPath := filepath.Join(nas, "obj", "existing.fit")
	writeFile(t, libPath, data)
	writeFile(t, filepath.Join(src, "dup.fit"), data)
	lib := newFakeLib()
	lib.add(t, libPath)

	c := candidate(src, nas, "dup.fit")
	p := Run(context.Background(), []Candidate{c}, lib, Options{DeleteAfterCopy: true}, nil)

	if exists(c.SourcePath) {
		t.Fatal("verified duplicate should be deleted from source")
	}
	if p.Skipped != 1 || p.Kept != 0 || len(p.Errors) != 0 {
		t.Errorf("progress = %+v", p)
	}
	if exists(c.DestPath) {
		t.Error("duplicate should not be copied")
	}
}

func TestRunDuplicateNotDeletedWithoutFlag(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	data := []byte("identical frame data")
	libPath := filepath.Join(nas, "obj", "existing.fit")
	writeFile(t, libPath, data)
	writeFile(t, filepath.Join(src, "dup.fit"), data)
	lib := newFakeLib()
	lib.add(t, libPath)

	c := candidate(src, nas, "dup.fit")
	p := Run(context.Background(), []Candidate{c}, lib, Options{}, nil)
	if !exists(c.SourcePath) || p.Skipped != 1 || p.Kept != 0 {
		t.Errorf("source kept=%v progress=%+v", exists(c.SourcePath), p)
	}
}

func TestRunCopiesAndDeletesVerified(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "a.fit"), []byte("frame a"))
	writeFile(t, filepath.Join(src, "b.fit"), []byte("frame b"))
	lib := newFakeLib()
	cs := []Candidate{candidate(src, nas, "a.fit"), candidate(src, nas, "b.fit")}

	p := Run(context.Background(), cs, lib, Options{DeleteAfterCopy: true}, nil)

	if p.Copied != 2 || p.Current != 2 || p.Total != 2 || p.Phase != PhaseDone {
		t.Errorf("progress = %+v", p)
	}
	for _, c := range cs {
		if exists(c.SourcePath) {
			t.Errorf("%s: source should be deleted after verified copy", c.RelativePath)
		}
		if got, _ := os.ReadFile(c.DestPath); len(got) == 0 {
			t.Errorf("%s: destination missing", c.RelativePath)
		}
	}
	if len(lib.imported) != 2 || lib.imported[0].FileHash == "" {
		t.Errorf("imported = %+v", lib.imported)
	}
}

func TestRunExistingDestDifferentContentKept(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	c := candidate(src, nas, "a.fit")
	writeFile(t, c.SourcePath, []byte("new content"))
	writeFile(t, c.DestPath, []byte("other content"))

	p := Run(context.Background(), []Candidate{c}, newFakeLib(), Options{DeleteAfterCopy: true}, nil)
	if !exists(c.SourcePath) {
		t.Fatal("source deleted although destination differs")
	}
	if p.Skipped != 1 || p.Kept != 1 {
		t.Errorf("progress = %+v", p)
	}
	if got, _ := os.ReadFile(c.DestPath); string(got) != "other content" {
		t.Error("existing destination was overwritten")
	}
}

func TestRunExistingDestIdenticalDeleted(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	c := candidate(src, nas, "a.fit")
	writeFile(t, c.SourcePath, []byte("same"))
	writeFile(t, c.DestPath, []byte("same"))

	p := Run(context.Background(), []Candidate{c}, newFakeLib(), Options{DeleteAfterCopy: true}, nil)
	if exists(c.SourcePath) || p.Skipped != 1 || p.Kept != 0 {
		t.Errorf("source exists=%v progress=%+v", exists(c.SourcePath), p)
	}
}

func TestRunDuplicateWithinBatch(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "a.fit"), []byte("same"))
	writeFile(t, filepath.Join(src, "sub", "b.fit"), []byte("same"))
	cs := []Candidate{candidate(src, nas, "a.fit"), candidate(src, nas, filepath.Join("sub", "b.fit"))}

	p := Run(context.Background(), cs, newFakeLib(), Options{DeleteAfterCopy: true}, nil)
	if p.Copied != 1 || p.Skipped != 1 || p.Kept != 0 {
		t.Errorf("progress = %+v", p)
	}
	if exists(cs[0].SourcePath) || exists(cs[1].SourcePath) {
		t.Error("both sources should be deleted (one copied, one verified duplicate)")
	}
}

func TestRunNeverDeletesSourceMatchingItself(t *testing.T) {
	// Importing from inside the library: the only "duplicate" is the source.
	dir := t.TempDir()
	path := filepath.Join(dir, "a.fit")
	writeFile(t, path, []byte("only copy"))
	lib := newFakeLib()
	lib.add(t, path)
	c := Candidate{SourcePath: path, RelativePath: "a.fit", DestPath: path}

	p := Run(context.Background(), []Candidate{c}, lib, Options{DeleteAfterCopy: true}, nil)
	if !exists(path) || p.Kept != 1 {
		t.Errorf("exists=%v progress=%+v", exists(path), p)
	}
}

func TestRunCollectsErrorsAndContinues(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	missing := candidate(src, nas, "missing.fit") // never written → hash fails
	ok := candidate(src, nas, "ok.fit")
	writeFile(t, ok.SourcePath, []byte("fine"))

	p := Run(context.Background(), []Candidate{missing, ok}, newFakeLib(), Options{}, nil)
	if p.Phase != PhaseDone || p.Copied != 1 || p.Current != 2 {
		t.Errorf("progress = %+v", p)
	}
	if len(p.Errors) != 1 || !strings.HasPrefix(p.Errors[0], "missing.fit:") {
		t.Errorf("errors = %v", p.Errors)
	}
	if !exists(ok.DestPath) {
		t.Error("file after the failing one was not copied")
	}
}

func TestRunCancel(t *testing.T) {
	src, nas := t.TempDir(), t.TempDir()
	var cs []Candidate
	for _, n := range []string{"a.fit", "b.fit", "c.fit"} {
		writeFile(t, filepath.Join(src, n), []byte(n))
		cs = append(cs, candidate(src, nas, n))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []Progress
	emit := func(p Progress) {
		events = append(events, p)
		if p.Phase == PhaseCopying && p.Current == 1 {
			cancel() // cancel while the second file is being processed
		}
	}

	p := Run(ctx, cs, newFakeLib(), Options{}, emit)
	if p.Phase != PhaseCancelled {
		t.Fatalf("phase = %s, want cancelled", p.Phase)
	}
	if p.Copied != 2 || p.Current != 2 {
		t.Errorf("copied/current = %d/%d, want 2/2", p.Copied, p.Current)
	}
	if exists(cs[2].DestPath) {
		t.Error("third file copied after cancellation")
	}
	if last := events[len(events)-1]; last.Phase != PhaseCancelled {
		t.Errorf("last event phase = %s", last.Phase)
	}
}

func TestIdenticalToAny(t *testing.T) {
	dir := t.TempDir()
	a, b := prefixCollision()
	src := filepath.Join(dir, "src")
	same := filepath.Join(dir, "same")
	diff := filepath.Join(dir, "diff")
	short := filepath.Join(dir, "short")
	writeFile(t, src, a)
	writeFile(t, same, a)
	writeFile(t, diff, b)
	writeFile(t, short, a[:10])

	if ok, err := identicalToAny(src, []string{diff, short, filepath.Join(dir, "nope")}); ok || err != nil {
		t.Errorf("non-identical: ok=%v err=%v", ok, err)
	}
	if ok, err := identicalToAny(src, []string{diff, same}); !ok || err != nil {
		t.Errorf("identical: ok=%v err=%v", ok, err)
	}
	if ok, _ := identicalToAny(src, []string{src}); ok {
		t.Error("source must not count as its own duplicate")
	}
}
