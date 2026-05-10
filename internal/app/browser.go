package app

import (
	"os"
	"path/filepath"
	"time"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	ModTime time.Time `json:"modTime"`
	Size    int64     `json:"size"`
}

// EnrichedFileEntry extends FileEntry with FITS header metadata loaded from
// the local cache. Fields are zero/empty when HasMeta is false.
type EnrichedFileEntry struct {
	FileEntry
	Object          string  `json:"object"`
	Filter          string  `json:"filter"`
	ExpTime         float64 `json:"expTime"`
	DateObs         string  `json:"dateObs"`
	Gain            float64 `json:"gain"`
	CCDTemp         float64 `json:"ccdTemp"`
	Telescope       string  `json:"telescope"`
	Instrument      string  `json:"instrument"`
	HasMeta         bool    `json:"hasMeta"`
	IsRejected      bool    `json:"isRejected"`
	RejectionReason string  `json:"rejectionReason"`
}

func (a *App) SelectRootFolder() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Astrophotography Root Folder",
	})
	if err != nil {
		return ""
	}
	return path
}

func (a *App) ListDirectory(path string) ([]FileEntry, error) {
	return listDirectory(path)
}

// ListDirectoryEnriched returns directory entries enriched with cached FITS
// header metadata. It is cache-only and returns immediately — files not yet in
// the cache will have HasMeta=false. Run BuildIndex to populate the cache.
func (a *App) ListDirectoryEnriched(path string) ([]EnrichedFileEntry, error) {
	entries, err := listDirectory(path)
	if err != nil {
		return nil, err
	}

	if a.prefs == nil {
		result := make([]EnrichedFileEntry, len(entries))
		for i, e := range entries {
			result[i] = EnrichedFileEntry{FileEntry: e}
		}
		return result, nil
	}

	allPaths := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir {
			allPaths = append(allPaths, e.Path)
		}
	}

	frames, err := a.prefs.GetFrames(allPaths)
	if err != nil {
		runtime.LogErrorf(a.ctx, "frames: get: %v", err)
		frames = map[string]prefs.Frame{}
	}

	result := make([]EnrichedFileEntry, len(entries))
	for i, e := range entries {
		ee := EnrichedFileEntry{FileEntry: e}
		if f, ok := frames[e.Path]; ok {
			ee.IsRejected = f.Rejected
			ee.RejectionReason = f.RejectionReason
			if f.CachedAt > 0 {
				ee.Object = f.Object
				ee.Filter = f.Filter
				ee.ExpTime = f.ExpTime
				ee.DateObs = f.DateObs
				ee.Gain = f.Gain
				ee.CCDTemp = f.CCDTemp
				ee.Telescope = f.Telescope
				ee.Instrument = f.Instrument
				ee.HasMeta = true
			}
		}
		result[i] = ee
	}
	return result, nil
}

func listDirectory(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	files := make([]FileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, FileEntry{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}
	return files, nil
}
