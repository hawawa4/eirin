//go:build !server

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/TaruDesigns/eirin/internal/importer"
)

// SelectSourceFolder opens an OS directory dialog for the user to pick the
// folder they want to import files from.
func (a *App) SelectSourceFolder() (string, error) {
	path, err := a.wails.Dialog.OpenFile().
		SetTitle("Select Source Folder to Import From").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

// ScanImportCandidates walks sourceFolder and returns files whose basename does
// not already appear in the frames database. extensions limits which file types
// are included (e.g. ["fit","fits","png"]); an empty slice includes everything.
// The destination path is flattened: nasRoot/immediateParentDir/filename.
// Note: this scan is basename-only for speed. The actual hash-based duplicate
// check happens during StartImport when files are read anyway.
func (a *App) ScanImportCandidates(sourceFolder string, extensions []string) ([]ImportCandidate, error) {
	nasRoot := a.store().Load().RootFolder
	if nasRoot == "" {
		return nil, fmt.Errorf("no NAS root folder configured")
	}

	known, err := a.store().GetAllFrameBasenames()
	if err != nil {
		return nil, fmt.Errorf("querying database: %w", err)
	}

	var candidates []ImportCandidate
	err = filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !importer.MatchesExtensions(info.Name(), extensions) {
			return nil
		}
		if known[info.Name()] {
			return nil
		}
		rel, relErr := filepath.Rel(sourceFolder, path)
		if relErr != nil {
			return nil
		}
		// Flatten: destination is nasRoot/immediateParentFolder/filename.
		// If the file sits directly in the source root, it lands in nasRoot directly.
		parentDir := filepath.Base(filepath.Dir(path))
		var destPath string
		if parentDir == "." || parentDir == filepath.Base(sourceFolder) {
			destPath = filepath.Join(nasRoot, info.Name())
		} else {
			destPath = filepath.Join(nasRoot, parentDir, info.Name())
		}
		candidates = append(candidates, ImportCandidate{
			SourcePath:   path,
			RelativePath: rel,
			DestPath:     destPath,
			FileSize:     info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return candidates, nil
}

// StartImport re-scans sourceFolder, then copies qualifying files to the NAS
// root in a background goroutine. extensions filters by file type (empty = all).
// When deleteAfterCopy is true, a source file is removed only once an
// identical copy (same size and full-file SHA-256) is confirmed in the library.
// FITS files are indexed into the database immediately after being copied.
// Progress is reported via "import:progress" events and GetImportStatus.
// Returns an error if an import is already running, the NAS root is not
// accessible, or the scan fails; per-file failures are reported in the final
// progress event's Errors instead.
func (a *App) StartImport(sourceFolder string, extensions []string, deleteAfterCopy bool) error {
	ctx, err := a.imports.begin()
	if err != nil {
		return err
	}
	fail := func(err error) error {
		a.imports.finish(ImportProgress{Phase: importer.PhaseError, Error: err.Error(), Errors: []string{}, Notes: []string{}})
		return err
	}

	nasRoot := a.store().Load().RootFolder
	if nasRoot == "" {
		return fail(errors.New("no NAS root folder configured"))
	}
	if info, err := os.Stat(nasRoot); err != nil || !info.IsDir() {
		return fail(fmt.Errorf("NAS root folder is not accessible: %s", nasRoot))
	}
	candidates, err := a.ScanImportCandidates(sourceFolder, extensions)
	if err != nil {
		return fail(err)
	}
	knownHashes, err := a.store().GetAllFrameHashes()
	if err != nil {
		return fail(fmt.Errorf("querying hashes: %w", err))
	}

	go a.runImport(ctx, candidates, knownHashes, deleteAfterCopy)
	return nil
}

func (a *App) emitImportProgress(p ImportProgress) {
	a.imports.update(p)
	a.emitEvent("import:progress", p)
}

// importLibrary adapts the store to importer.Library.
type importLibrary struct {
	app         *App
	knownHashes map[string]bool
}

func (l importLibrary) HasPrefixHash(hash string) bool { return l.knownHashes[hash] }

func (l importLibrary) PathsWithPrefixHash(hash string) ([]string, error) {
	return l.app.store().GetFramePathsByHash(hash)
}

func (l importLibrary) FileImported(c ImportCandidate) { l.app.indexImportedFile(c) }

func (a *App) runImport(ctx context.Context, candidates []ImportCandidate, knownHashes map[string]bool, deleteAfterCopy bool) {
	lib := importLibrary{app: a, knownHashes: knownHashes}
	final := importer.Run(ctx, candidates, lib, importer.Options{DeleteAfterCopy: deleteAfterCopy}, a.emitImportProgress)
	a.imports.finish(final)
	slog.Info("import: finished", "phase", final.Phase, "copied", final.Copied,
		"skipped", final.Skipped, "kept", final.Kept, "errors", len(final.Errors))

	if final.Copied > 0 {
		a.emitEvent("library:updated", nil)
	}
}
