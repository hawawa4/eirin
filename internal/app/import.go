package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/importer"
	"github.com/TaruDesigns/eirin/internal/indexer"
	"github.com/TaruDesigns/eirin/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ImportCandidate is a file in the source folder not yet in the library.
// Re-exported from the importer package for Wails binding compatibility.
type ImportCandidate = importer.Candidate

// ImportProgress is emitted as an "import:progress" event during StartImport.
// Re-exported from the importer package for Wails binding compatibility.
type ImportProgress = importer.Progress

// SelectSourceFolder opens an OS directory dialog for the user to pick the
// folder they want to import files from.
func (a *App) SelectSourceFolder() (string, error) {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Source Folder to Import From",
	})
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
	nasRoot := a.store.Load().RootFolder
	if nasRoot == "" {
		return nil, fmt.Errorf("no NAS root folder configured")
	}

	known, err := a.store.GetAllFrameBasenames()
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
// When deleteAfterCopy is true, each source file is removed after a successful copy.
// FITS files are indexed into the database immediately after being copied.
// Progress is reported via "import:progress" events.
func (a *App) StartImport(sourceFolder string, extensions []string, deleteAfterCopy bool) error {
	candidates, err := a.ScanImportCandidates(sourceFolder, extensions)
	if err != nil {
		return err
	}
	go a.runImport(candidates, deleteAfterCopy)
	return nil
}

func (a *App) runImport(candidates []ImportCandidate, deleteAfterCopy bool) {
	total := len(candidates)
	copied, skipped := 0, 0

	knownHashes, err := a.store.GetAllFrameHashes()
	if err != nil {
		runtime.EventsEmit(a.ctx, "import:progress", ImportProgress{
			Phase: "error",
			Total: total,
			Error: fmt.Sprintf("querying hashes: %v", err),
		})
		return
	}

	for i, c := range candidates {
		runtime.EventsEmit(a.ctx, "import:progress", ImportProgress{
			Phase:       "copying",
			Current:     i,
			Total:       total,
			CurrentFile: c.RelativePath,
			Copied:      copied,
			Skipped:     skipped,
		})

		hash, hashErr := importer.HashFilePrefix(c.SourcePath)
		if hashErr != nil {
			runtime.EventsEmit(a.ctx, "import:progress", ImportProgress{
				Phase:   "error",
				Current: i,
				Total:   total,
				Error:   fmt.Sprintf("%s: %v", c.RelativePath, hashErr),
			})
			return
		}

		if knownHashes[hash] {
			// Same content already in library — skip copy but honour delete-from-source.
			skipped++
			if deleteAfterCopy {
				if rmErr := os.Remove(c.SourcePath); rmErr != nil {
					runtime.LogWarningf(a.ctx, "import: delete source %s: %v", c.RelativePath, rmErr)
				}
			}
			continue
		}

		wasSkipped, copyErr := importer.CopyFileIfNotExists(c.SourcePath, c.DestPath)
		if copyErr != nil {
			runtime.EventsEmit(a.ctx, "import:progress", ImportProgress{
				Phase:   "error",
				Current: i,
				Total:   total,
				Error:   fmt.Sprintf("%s: %v", c.RelativePath, copyErr),
			})
			return
		}

		if wasSkipped {
			skipped++
		} else {
			knownHashes[hash] = true // guard against duplicates within the same import batch
			copied++
			c.FileHash = hash
			a.indexImportedFile(c)
			if deleteAfterCopy {
				if rmErr := os.Remove(c.SourcePath); rmErr != nil {
					runtime.LogWarningf(a.ctx, "import: delete source %s: %v", c.RelativePath, rmErr)
				}
			}
		}
	}

	runtime.EventsEmit(a.ctx, "import:progress", ImportProgress{
		Phase:   "done",
		Current: total,
		Total:   total,
		Copied:  copied,
		Skipped: skipped,
	})
}

// inferObjectForRaster resolves the object name for a raster file by querying
// the DB for any indexed frame (typically a light) under the same directory.
// Falls back to the directory name when nothing is indexed there yet.
func (a *App) inferObjectForRaster(path string) string {
	dir := filepath.Dir(path)
	if obj, err := a.store.GetObjectForDirectory(dir); err == nil && obj != "" {
		return obj
	}
	return filepath.Base(dir)
}

// indexImportedFile inserts a freshly-copied file into the database so it
// shows up in the library without a separate Build Index run.
// FITS files are parsed for header metadata; PNG/TIFF are stored directly as
// processed frames with no header data.
func (a *App) indexImportedFile(c ImportCandidate) {
	switch {
	case indexer.IsFitsFile(c.DestPath):
		hdr, err := fits.ReadFITSHeader(c.DestPath)
		if err != nil {
			runtime.LogWarningf(a.ctx, "import: index %s: %v", c.RelativePath, err)
			return
		}
		frame := store.Frame{
			FileSize:   c.FileSize,
			LastSeen:   time.Now().Unix(),
			FileHash:   c.FileHash,
			Object:     hdr.Object,
			Filter:     hdr.Filter,
			ExpTime:    hdr.ExpTime,
			DateObs:    hdr.DateObs,
			Gain:       hdr.Gain,
			CCDTemp:    hdr.CCDTemp,
			Telescope:  hdr.Telescope,
			Instrument: hdr.Instrument,
			FrameType:  store.ClassifyFrameType(c.DestPath),
		}
		if err := a.store.UpsertFrame(c.DestPath, frame); err != nil {
			runtime.LogWarningf(a.ctx, "import: upsert %s: %v", c.DestPath, err)
		}
	case indexer.IsRasterFile(c.DestPath):
		frame := store.Frame{
			FileSize:  c.FileSize,
			LastSeen:  time.Now().Unix(),
			FileHash:  c.FileHash,
			FrameType: store.FrameTypeProcessed,
			Object:    a.inferObjectForRaster(c.DestPath),
		}
		if err := a.store.UpsertFrame(c.DestPath, frame); err != nil {
			runtime.LogWarningf(a.ctx, "import: upsert %s: %v", c.DestPath, err)
		}
		go func() {
			if err := a.AnalyzeFrames([]string{c.DestPath}); err != nil {
				runtime.LogWarningf(a.ctx, "import: analyze %s: %v", c.RelativePath, err)
			}
		}()
	}
}
