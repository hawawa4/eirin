package browser

import (
	"os"
	"path/filepath"
	"time"
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
	Object     string  `json:"object"`
	Filter     string  `json:"filter"`
	ExpTime    float64 `json:"expTime"`
	DateObs    string  `json:"dateObs"`
	Gain       float64 `json:"gain"`
	CCDTemp    float64 `json:"ccdTemp"`
	Telescope  string  `json:"telescope"`
	Instrument string  `json:"instrument"`
	HasMeta    bool    `json:"hasMeta"`
	IsRejected bool    `json:"isRejected"`
}

func ListDirectory(path string) ([]FileEntry, error) {
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
