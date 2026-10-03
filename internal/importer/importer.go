package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const hashPrefixBytes = 1 << 20 // 1 MiB

// HashFilePrefix returns a hex-encoded SHA-256 of the first 1 MiB of the file.
func HashFilePrefix(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, hashPrefixBytes)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFileFull returns a hex-encoded SHA-256 of the entire file, streamed so
// large files are never held in memory.
func HashFileFull(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Candidate is a file in the source folder not yet in the library.
type Candidate struct {
	SourcePath   string `json:"sourcePath"`
	RelativePath string `json:"relativePath"` // full path relative to source root (for tree display)
	DestPath     string `json:"destPath"`     // flattened: nasRoot/immediateParentDir/filename
	FileSize     int64  `json:"fileSize"`
	FileHash     string `json:"fileHash,omitempty"` // SHA-256 of first 1 MiB; set during import
}

// Import phases reported in Progress.Phase.
const (
	PhaseIdle      = "idle"      // no import has run since startup
	PhaseScanning  = "scanning"  // StartImport is scanning the source folder
	PhaseCopying   = "copying"   // files are being processed
	PhaseDone      = "done"      // run finished (possibly with per-file Errors)
	PhaseCancelled = "cancelled" // run stopped by CancelImport
	PhaseError     = "error"     // run aborted by a fatal error (see Error)
)

// Progress is emitted during an import run.
type Progress struct {
	Phase       string `json:"phase"`   // see Phase* constants
	Current     int    `json:"current"` // files fully processed so far (reaches Total at the end)
	Total       int    `json:"total"`
	CurrentFile string `json:"currentFile"`
	Copied      int    `json:"copied"`
	Skipped     int    `json:"skipped"` // already in the library (by content or destination path)
	// Kept counts source files NOT deleted from the device although
	// deleteAfterCopy was on, because the library copy could not be verified
	// identical (size + full SHA-256).
	Kept int `json:"kept"`
	// Errors lists per-file failures ("relative/path: reason"); the run
	// continues past them.
	Errors []string `json:"errors"`
	// Notes lists per-file informational messages, e.g. why a file was kept.
	Notes []string `json:"notes"`
	Error string   `json:"error,omitempty"` // fatal error (Phase == "error")
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
// The destination is created exclusively, and a partially written destination
// is removed if the copy fails.
func CopyFileIfNotExists(src, dst string) (skipped bool, err error) {
	if _, statErr := os.Lstat(dst); statErr == nil {
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
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return true, nil
		}
		return false, err
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(dst)
		return false, err
	}
	return false, nil
}
