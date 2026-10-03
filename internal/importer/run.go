package importer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// noteNotVerified is recorded when a source file is kept on the device because
// its library copy could not be proven identical.
const noteNotVerified = "kept on device: not verified identical"

// hashFull computes a file's full SHA-256; a variable so tests can inject
// read failures.
var hashFull = HashFileFull

// Library is the view of the existing frame catalog that Run needs.
type Library interface {
	// HasPrefixHash reports whether a library frame has this 1 MiB prefix hash.
	HasPrefixHash(hash string) bool
	// PathsWithPrefixHash returns library file paths whose prefix hash
	// matches. Only used to verify duplicates before deleting a source file.
	PathsWithPrefixHash(hash string) ([]string, error)
	// FileImported is called after a candidate has been copied to DestPath
	// (with FileHash set), so it can be indexed.
	FileImported(c Candidate)
}

// Options configures a Run.
type Options struct {
	// DeleteAfterCopy removes a source file once an identical copy (same size
	// and full-file SHA-256) is confirmed to exist in the library.
	DeleteAfterCopy bool
}

// Run processes candidates one by one: content-duplicates (by prefix hash)
// are skipped, everything else is copied to its DestPath. Per-file failures
// are collected in Progress.Errors and the run continues. ctx is checked
// between files; on cancellation the final progress has Phase "cancelled".
//
// emit (may be nil) receives a progress snapshot before each file and the
// final snapshot, which is also returned.
func Run(ctx context.Context, candidates []Candidate, lib Library, opts Options, emit func(Progress)) Progress {
	r := runner{lib: lib, opts: opts, batch: map[string][]string{}}
	r.p = Progress{Phase: PhaseCopying, Total: len(candidates), Errors: []string{}, Notes: []string{}}
	send := func() {
		if emit != nil {
			emit(r.snapshot())
		}
	}

	for _, c := range candidates {
		if ctx.Err() != nil {
			r.p.Phase = PhaseCancelled
			r.p.CurrentFile = ""
			send()
			return r.snapshot()
		}
		r.p.CurrentFile = c.RelativePath
		send()
		r.process(c)
		r.p.Current++
	}

	r.p.Phase = PhaseDone
	r.p.CurrentFile = ""
	send()
	return r.snapshot()
}

type runner struct {
	lib  Library
	opts Options
	p    Progress
	// batch maps prefix hash → destinations copied earlier in this run, so
	// duplicates within the same import are detected and verifiable.
	batch map[string][]string
}

// snapshot returns a copy of the progress whose slices don't alias the
// runner's (events may be serialised on another goroutine).
func (r *runner) snapshot() Progress {
	p := r.p
	p.Errors = append([]string{}, r.p.Errors...)
	p.Notes = append([]string{}, r.p.Notes...)
	return p
}

func (r *runner) fail(c Candidate, format string, args ...any) {
	msg := fmt.Sprintf("%s: %s", c.RelativePath, fmt.Sprintf(format, args...))
	slog.Warn("import: file failed", "err", msg)
	r.p.Errors = append(r.p.Errors, msg)
}

func (r *runner) process(c Candidate) {
	hash, err := HashFilePrefix(c.SourcePath)
	if err != nil {
		r.fail(c, "hashing: %v", err)
		return
	}

	if r.lib.HasPrefixHash(hash) || len(r.batch[hash]) > 0 {
		// Same leading content already in the library — don't copy. The prefix
		// hash alone is not proof of identity, so deletion re-verifies.
		r.p.Skipped++
		if r.opts.DeleteAfterCopy {
			paths, err := r.lib.PathsWithPrefixHash(hash)
			if err != nil {
				r.fail(c, "looking up duplicates: %v", err)
				return
			}
			r.deleteIfVerified(c, append(paths, r.batch[hash]...))
		}
		return
	}

	dest, duplicate, err := copyToFreeName(c.SourcePath, c.DestPath)
	if err != nil {
		r.fail(c, "copying: %v", err)
		return
	}
	if duplicate {
		// An identical file (size + full SHA-256) already sits at dest — a
		// genuine duplicate not known by hash (e.g. not indexed yet).
		r.p.Skipped++
		if r.opts.DeleteAfterCopy {
			r.deleteIfVerified(c, []string{dest})
		}
		return
	}
	if dest != c.DestPath {
		// The flattened destination name was taken by a different file
		// (e.g. two device folders with the same name and file names).
		r.p.Notes = append(r.p.Notes, fmt.Sprintf("%s: saved as %s (name already taken by a different file)", c.RelativePath, filepath.Base(dest)))
		c.DestPath = dest
	}

	r.p.Copied++
	r.batch[hash] = append(r.batch[hash], c.DestPath)
	c.FileHash = hash
	r.lib.FileImported(c)
	if r.opts.DeleteAfterCopy {
		r.deleteIfVerified(c, []string{c.DestPath})
	}
}

// deleteIfVerified removes the source file only if one of libraryPaths is a
// different file with the same size and full-file SHA-256.
func (r *runner) deleteIfVerified(c Candidate, libraryPaths []string) {
	ok, err := identicalToAny(c.SourcePath, libraryPaths)
	if err != nil {
		// The error already explains why; the file stays on the device.
		r.fail(c, "verifying: %v", err)
		r.p.Kept++
		return
	}
	if !ok {
		r.p.Kept++
		r.p.Notes = append(r.p.Notes, fmt.Sprintf("%s: %s", c.RelativePath, noteNotVerified))
		return
	}
	if err := os.Remove(c.SourcePath); err != nil {
		r.fail(c, "deleting source: %v", err)
	}
}

// identicalToAny reports whether any of paths is a distinct file with the
// same size and full-file SHA-256 as src. The source's full hash is computed
// lazily, only once a size-matching candidate is found. Unreadable candidates
// are ignored; an error is returned only if src itself can't be read.
func identicalToAny(src string, paths []string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	srcHash := ""
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() || info.Size() != srcInfo.Size() {
			continue
		}
		if os.SameFile(srcInfo, info) {
			// The "library copy" is the source itself — deleting it would
			// lose the only copy.
			continue
		}
		if srcHash == "" {
			if srcHash, err = hashFull(src); err != nil {
				return false, err
			}
		}
		h, err := hashFull(p)
		if err != nil {
			slog.Warn("import: hashing library file", "path", p, "err", err)
			continue
		}
		if h == srcHash {
			return true, nil
		}
	}
	return false, nil
}

// maxSuffix bounds the name_N search in copyToFreeName.
const maxSuffix = 10000

// copyToFreeName copies src to dst, or — when dst is already taken by a
// different file — to the first free suffixed name (name_2.fit, name_3.fit,
// …). If dst (or a suffixed slot tried along the way) already holds a file
// identical to src (same size and full SHA-256), nothing is copied and
// duplicate is true; dest is then that identical file. Existing files are
// never overwritten.
func copyToFreeName(src, dst string) (dest string, duplicate bool, err error) {
	dir := filepath.Dir(dst)
	base := filepath.Base(dst)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	srcHash := ""
	for n := 1; n < maxSuffix; n++ {
		dest = dst
		if n > 1 {
			dest = filepath.Join(dir, stem+"_"+strconv.Itoa(n)+ext)
		}
		skipped, err := CopyFileIfNotExists(src, dest)
		if err != nil {
			return "", false, err
		}
		if !skipped {
			return dest, false, nil
		}
		same, err := sameContent(src, dest, &srcHash)
		if err != nil {
			return "", false, fmt.Errorf("comparing with existing %s: %w", filepath.Base(dest), err)
		}
		if same {
			return dest, true, nil
		}
	}
	return "", false, fmt.Errorf("too many files named %s in %s", base, dir)
}

// sameContent reports whether the existing path dst holds the same content as
// src: the very same file, or a regular file with the same size and full-file
// SHA-256. A dangling or non-regular dst is "different". *srcHash memoizes the
// source's full hash across calls ("" = not computed yet).
func sameContent(src, dst string, srcHash *string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(dst)
	if err != nil || !info.Mode().IsRegular() {
		return false, nil
	}
	if os.SameFile(srcInfo, info) {
		return true, nil
	}
	if info.Size() != srcInfo.Size() {
		return false, nil
	}
	if *srcHash == "" {
		if *srcHash, err = hashFull(src); err != nil {
			return false, err
		}
	}
	h, err := hashFull(dst)
	if err != nil {
		return false, err
	}
	return h == *srcHash, nil
}
