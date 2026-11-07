package app

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
)

// pruneMissingFrames deletes the frame records under rootPath whose file no
// longer exists, so frames removed outside Eirin don't linger in the library.
// seen is every indexable path the scan just found on disk.
//
// A vanished NAS mount looks like an empty (or unreadable) root, and pruning
// then would wipe the library's metadata, so the caller only runs this after
// a fully readable walk, and it does nothing when the walk found no file at
// all. Every candidate is also stat'ed individually and only removed on a
// definite "does not exist" (not on a timeout or permission error), which
// also keeps files reached through symlinked folders the walk doesn't follow.
// Returns how many records were removed.
func (a *App) pruneMissingFrames(ctx context.Context, rootPath string, seen []string) int {
	if len(seen) == 0 {
		return 0
	}
	known, err := a.store().GetAllPathsUnder(rootPath)
	if err != nil {
		slog.Warn("index: list paths for prune", "err", err)
		return 0
	}
	onDisk := make(map[string]struct{}, len(seen))
	for _, p := range seen {
		onDisk[p] = struct{}{}
	}
	var gone []string
	for _, p := range known {
		if ctx.Err() != nil {
			return 0
		}
		if _, ok := onDisk[p]; ok {
			continue
		}
		if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			gone = append(gone, p)
		}
	}
	if len(gone) == 0 {
		return 0
	}
	if err := a.store().BatchDeleteFrames(gone); err != nil {
		slog.Warn("index: prune missing frames", "err", err)
		return 0
	}
	slog.Info("index: removed records of missing files", "count", len(gone))
	return len(gone)
}
