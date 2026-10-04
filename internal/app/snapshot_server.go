//go:build server

package app

// GetSnapshotStatus reports which library snapshot this server is showing.
func (a *App) GetSnapshotStatus() SnapshotStatus {
	if a.snapshots.loader == nil {
		return SnapshotStatus{}
	}
	s := a.snapshots.loader.Status()
	return SnapshotStatus{
		Root:        s.Root,
		Loaded:      s.Loaded,
		SourceRoot:  s.SourceRoot,
		PublishedAt: formatTime(s.PublishedAt),
		LoadedAt:    formatTime(s.LoadedAt),
		Frames:      s.Frames,
		LastError:   s.LastError,
	}
}

// PublishSnapshot is a desktop action; the server only reads snapshots.
func (a *App) PublishSnapshot() error {
	return errReadOnly
}
