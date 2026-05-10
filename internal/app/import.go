package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ImportCandidate is a file in the source folder not yet in the library.
type ImportCandidate struct {
	SourcePath   string `json:"sourcePath"`
	RelativePath string `json:"relativePath"` // full path relative to source root (for tree display)
	DestPath     string `json:"destPath"`     // flattened: nasRoot/immediateParentDir/filename
	FileSize     int64  `json:"fileSize"`
}

// ImportProgress is emitted as an "import:progress" event during StartImport.
type ImportProgress struct {
	Phase       string `json:"phase"` // "copying" | "done" | "error"
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CurrentFile string `json:"currentFile"`
	Copied      int    `json:"copied"`
	Skipped     int    `json:"skipped"`
	Error       string `json:"error,omitempty"`
}

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

// ScanImportCandidates walks sourceFolder and returns all files whose basename
// does not already appear in the frames database. The destination path is
// flattened: nasRoot/immediateParentDir/filename (one level of subdirectory).
func (a *App) ScanImportCandidates(sourceFolder string) ([]ImportCandidate, error) {
	nasRoot := a.prefs.Load().RootFolder
	if nasRoot == "" {
		return nil, fmt.Errorf("no NAS root folder configured")
	}

	known, err := a.prefs.GetAllFrameBasenames()
	if err != nil {
		return nil, fmt.Errorf("querying database: %w", err)
	}

	var candidates []ImportCandidate
	err = filepath.Walk(sourceFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
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

// StartImport re-scans sourceFolder, then copies all new files to the NAS
// root in a background goroutine. FITS files are indexed into the database
// immediately after being copied. Progress is reported via "import:progress" events.
func (a *App) StartImport(sourceFolder string) error {
	candidates, err := a.ScanImportCandidates(sourceFolder)
	if err != nil {
		return err
	}
	go a.runImport(candidates)
	return nil
}

func (a *App) runImport(candidates []ImportCandidate) {
	total := len(candidates)
	copied, skipped := 0, 0

	for i, c := range candidates {
		runtime.EventsEmit(a.ctx, "import:progress", ImportProgress{
			Phase:       "copying",
			Current:     i,
			Total:       total,
			CurrentFile: c.RelativePath,
			Copied:      copied,
			Skipped:     skipped,
		})

		wasSkipped, err := copyFileIfNotExists(c.SourcePath, c.DestPath)
		if err != nil {
			runtime.EventsEmit(a.ctx, "import:progress", ImportProgress{
				Phase:   "error",
				Current: i,
				Total:   total,
				Error:   fmt.Sprintf("%s: %v", c.RelativePath, err),
			})
			return
		}

		if wasSkipped {
			skipped++
		} else {
			copied++
			a.indexImportedFile(c)
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

// indexImportedFile reads the FITS header of a freshly-copied file and inserts
// it into the database, so it shows up in the library without a separate Build Index run.
func (a *App) indexImportedFile(c ImportCandidate) {
	if !isFitsFile(c.DestPath) {
		return
	}
	hdr, err := readFITSHeader(c.DestPath)
	if err != nil {
		runtime.LogWarningf(a.ctx, "import: index %s: %v", c.RelativePath, err)
		return
	}
	frame := prefs.Frame{
		FileSize:   c.FileSize,
		LastSeen:   time.Now().Unix(),
		Object:     hdr.Object,
		Filter:     hdr.Filter,
		ExpTime:    hdr.ExpTime,
		DateObs:    hdr.DateObs,
		Gain:       hdr.Gain,
		CCDTemp:    hdr.CCDTemp,
		Telescope:  hdr.Telescope,
		Instrument: hdr.Instrument,
		FrameType:  prefs.ClassifyFrameType(c.DestPath),
	}
	if err := a.prefs.UpsertFrame(c.DestPath, frame); err != nil {
		runtime.LogWarningf(a.ctx, "import: upsert %s: %v", c.DestPath, err)
	}
}

// copyFileIfNotExists copies src to dst, creating parent directories as needed.
// Returns (true, nil) if dst already existed and was skipped, or (false, nil/err).
func copyFileIfNotExists(src, dst string) (skipped bool, err error) {
	if _, statErr := os.Stat(dst); statErr == nil {
		return true, nil
	}
	if mkErr := os.MkdirAll(filepath.Dir(dst), 0o755); mkErr != nil {
		return false, mkErr
	}
	in, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return false, err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return false, err
}
