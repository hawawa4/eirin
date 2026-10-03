package importer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// noteNotVerified is recorded when a source file is kept on the device because
// its library copy could not be proven identical.
const noteNotVerified = "kept on device: not verified identical"

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

	wasSkipped, err := CopyFileIfNotExists(c.SourcePath, c.DestPath)
	if err != nil {
		r.fail(c, "copying: %v", err)
		return
	}
	if wasSkipped {
		// Destination path already taken (by a file not known by hash).
		r.p.Skipped++
		if r.opts.DeleteAfterCopy {
			r.deleteIfVerified(c, []string{c.DestPath})
		}
		return
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
		r.fail(c, "verifying: %v", err)
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
			if srcHash, err = HashFileFull(src); err != nil {
				return false, err
			}
		}
		h, err := HashFileFull(p)
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
