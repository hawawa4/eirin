// Package projectfs manages the on-disk layout of Siril project folders:
// frames are linked (or copied) from the NAS into Siril-conventional
// subfolders — lights/, darks/, flats/, biases/ — and processed outputs live
// directly in the project root.
package projectfs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TaruDesigns/eirin/internal/importer"
	"github.com/TaruDesigns/eirin/internal/store"
)

// Frame subfolder names, following Siril's script conventions.
const (
	DirLights = "lights"
	DirDarks  = "darks"
	DirFlats  = "flats"
	DirBiases = "biases"
)

// Subdirs lists every frame subfolder of a project, lights first.
var Subdirs = []string{DirLights, DirDarks, DirFlats, DirBiases}

// Link modes accepted by AddFrame.
const (
	ModeSymlink = "symlink"
	ModeCopy    = "copy"
)

var subdirByType = map[string]string{
	store.FrameTypeLight: DirLights,
	store.FrameTypeDark:  DirDarks,
	store.FrameTypeFlat:  DirFlats,
	store.FrameTypeBias:  DirBiases,
}

// SubdirFor returns the project subfolder for a frame type. ok is false for
// types that don't belong in a Siril project (stacked, processed, image, "").
func SubdirFor(frameType string) (dir string, ok bool) {
	dir, ok = subdirByType[frameType]
	return dir, ok
}

// FrameTypeForSubdir is the inverse of SubdirFor ("" for unknown folders).
func FrameTypeForSubdir(dir string) string {
	for t, d := range subdirByType {
		if d == dir {
			return t
		}
	}
	return ""
}

// IsFrameSubdir reports whether name is one of the frame subfolders.
func IsFrameSubdir(name string) bool {
	return FrameTypeForSubdir(name) != ""
}

// ValidMode reports whether mode is a supported AddFrame mode.
func ValidMode(mode string) bool {
	return mode == ModeSymlink || mode == ModeCopy
}

// Create makes a new project folder with all frame subfolders. It fails if
// folder already exists, so two project names that sanitize to the same
// folder are never silently merged.
func Create(folder string) error {
	if err := os.MkdirAll(filepath.Dir(folder), 0o750); err != nil {
		return fmt.Errorf("creating projects folder: %w", err)
	}
	if err := os.Mkdir(folder, 0o750); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("a folder named %q already exists in the projects folder — choose a different project name", filepath.Base(folder))
		}
		return fmt.Errorf("creating project folder: %w", err)
	}
	for _, d := range Subdirs {
		if err := os.Mkdir(filepath.Join(folder, d), 0o750); err != nil {
			return fmt.Errorf("creating %s/: %w", d, err)
		}
	}
	return nil
}

// AddResult describes the outcome of AddFrame.
type AddResult struct {
	Path    string // project-local path of the (new or existing) entry
	Existed bool   // the same source was already present; nothing was written
}

// AddFrame links or copies src into projectFolder/subdir. If an entry with
// the same name exists and refers to the same source (same file through the
// symlink, or identical content for a copy) it is a no-op; if it refers to a
// different source, the frame is added under a numeric suffix (name_2.fit,
// name_3.fit, …) instead of replacing the existing entry.
func AddFrame(projectFolder, subdir, src, mode string) (AddResult, error) {
	if !ValidMode(mode) {
		return AddResult{}, fmt.Errorf("unknown mode %q — use symlink or copy", mode)
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		return AddResult{}, fmt.Errorf("source not accessible: %w", err)
	}
	dir := filepath.Join(projectFolder, subdir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return AddResult{}, fmt.Errorf("creating %s/: %w", subdir, err)
	}

	base := filepath.Base(src)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for n := 1; n < 10000; n++ {
		name := base
		if n > 1 {
			name = stem + "_" + strconv.Itoa(n) + ext
		}
		dst := filepath.Join(dir, name)
		lst, err := os.Lstat(dst)
		if errors.Is(err, os.ErrNotExist) {
			if err := place(src, dst, mode); err != nil {
				return AddResult{}, err
			}
			return AddResult{Path: dst}, nil
		}
		if err != nil {
			return AddResult{}, err
		}
		if lst.Mode()&os.ModeSymlink != 0 {
			if _, statErr := os.Stat(dst); statErr != nil {
				// Dangling link (its NAS file was moved/deleted): reuse the slot.
				if err := os.Remove(dst); err != nil {
					return AddResult{}, err
				}
				if err := place(src, dst, mode); err != nil {
					return AddResult{}, err
				}
				return AddResult{Path: dst}, nil
			}
		}
		if sameSource(dst, src, srcInfo) {
			return AddResult{Path: dst, Existed: true}, nil
		}
	}
	return AddResult{}, fmt.Errorf("too many files named %s in %s/", base, subdir)
}

func place(src, dst, mode string) error {
	if mode == ModeSymlink {
		return os.Symlink(src, dst)
	}
	return CopyNoOverwrite(src, dst)
}

// sameSource reports whether the existing entry dst refers to src: either it
// resolves to the very same file (symlink), or it is a regular file with the
// same size and full-file SHA-256 (copy).
func sameSource(dst, src string, srcInfo os.FileInfo) bool {
	info, err := os.Stat(dst)
	if err != nil {
		return false
	}
	if os.SameFile(info, srcInfo) {
		return true
	}
	if !info.Mode().IsRegular() || info.Size() != srcInfo.Size() {
		return false
	}
	h1, err := importer.HashFileFull(dst)
	if err != nil {
		return false
	}
	h2, err := importer.HashFileFull(src)
	if err != nil {
		return false
	}
	return h1 == h2
}

// Entry is one frame file found in a project's frame subfolders.
type Entry struct {
	Subdir     string // lights, darks, flats or biases
	Name       string // file name inside Subdir
	Path       string // project-local path (symlink or copy)
	SourcePath string // symlink target resolved to the NAS path; Path for copies or broken links
}

// ListFrames returns every file in the project's frame subfolders. Legacy
// projects that kept darks/biases in lights/ are listed as-is; callers should
// use the frame type from the DB, not Subdir, as the source of truth.
func ListFrames(projectFolder string) ([]Entry, error) {
	var out []Entry
	for _, sub := range Subdirs {
		dir := filepath.Join(projectFolder, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return out, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			p := filepath.Join(dir, e.Name())
			real, err := filepath.EvalSymlinks(p)
			if err != nil {
				real = p
			}
			out = append(out, Entry{Subdir: sub, Name: e.Name(), Path: p, SourcePath: real})
		}
	}
	return out, nil
}

// RemoveFrames deletes the project entries (symlinks or copies) that refer to
// any of paths. A path matches an entry if it equals the entry's project-local
// path or its resolved source. As a fallback for old copy-mode projects, a
// path that matched nothing removes regular files with the same basename.
// NAS files are never touched. Returns the number of removal failures.
func RemoveFrames(projectFolder string, paths []string) (failed int, err error) {
	entries, err := ListFrames(projectFolder)
	if err != nil {
		return 0, err
	}
	removed := map[string]bool{}
	remove := func(e Entry) {
		if removed[e.Path] {
			return
		}
		removed[e.Path] = true
		if err := os.Remove(e.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("remove from project", "file", e.Path, "err", err)
			failed++
		}
	}
	for _, p := range paths {
		keys := map[string]bool{p: true}
		if real, err := filepath.EvalSymlinks(p); err == nil {
			keys[real] = true
		}
		hit := false
		for _, e := range entries {
			if keys[e.Path] || keys[e.SourcePath] {
				hit = true
				remove(e)
			}
		}
		if hit {
			continue
		}
		for _, e := range entries {
			if e.Name != filepath.Base(p) {
				continue
			}
			if lst, err := os.Lstat(e.Path); err == nil && lst.Mode().IsRegular() {
				remove(e)
			}
		}
	}
	return failed, nil
}

// CopyNoOverwrite copies src to dst, failing if dst already exists. A partial
// destination is removed if the copy fails.
func CopyNoOverwrite(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(dst)
	}
	return err
}
