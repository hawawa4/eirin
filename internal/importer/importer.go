package importer

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Candidate is a file in the source folder not yet in the library.
type Candidate struct {
	SourcePath   string `json:"sourcePath"`
	RelativePath string `json:"relativePath"` // full path relative to source root (for tree display)
	DestPath     string `json:"destPath"`     // flattened: nasRoot/immediateParentDir/filename
	FileSize     int64  `json:"fileSize"`
}

// Progress is emitted during an import run.
type Progress struct {
	Phase       string `json:"phase"` // "copying" | "done" | "error"
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CurrentFile string `json:"currentFile"`
	Copied      int    `json:"copied"`
	Skipped     int    `json:"skipped"`
	Error       string `json:"error,omitempty"`
}

// MatchesExtensions reports whether name has one of the given extensions
// (case-insensitive, without leading dot). An empty slice matches everything.
func MatchesExtensions(name string, exts []string) bool {
	if len(exts) == 0 {
		return true
	}
	lower := strings.ToLower(filepath.Ext(name))
	if lower != "" {
		lower = lower[1:] // strip leading dot
	}
	for _, e := range exts {
		if lower == strings.ToLower(e) {
			return true
		}
	}
	return false
}

// CopyFileIfNotExists copies src to dst, creating parent directories as needed.
// Returns (true, nil) if dst already existed and was skipped, or (false, nil/err).
func CopyFileIfNotExists(src, dst string) (skipped bool, err error) {
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
