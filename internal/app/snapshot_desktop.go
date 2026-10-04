//go:build !server

package app

import "errors"

// GetSnapshotStatus reports whether the library snapshot is being published,
// where, and how the last attempt went.
func (a *App) GetSnapshotStatus() SnapshotStatus {
	if a.snapshots.publisher == nil {
		return SnapshotStatus{}
	}
	s := a.snapshots.publisher.Status()
	return SnapshotStatus{
		Enabled:       s.Enabled,
		Path:          s.Path,
		LastPublished: formatTime(s.LastPublished),
		LastError:     s.LastError,
	}
}

// PublishSnapshot publishes the library snapshot now, whether or not anything
// changed.
func (a *App) PublishSnapshot() error {
	if a.snapshots.publisher == nil {
		return errors.New("snapshot publishing is not running")
	}
	return a.snapshots.publisher.PublishNow()
}
