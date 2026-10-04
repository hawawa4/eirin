package app

import "time"

// SnapshotStatus describes the library snapshot shared between the desktop
// app and the read-only server viewer. The desktop fills the publishing
// fields, the server the loading ones. Times are RFC 3339, empty if unset.
type SnapshotStatus struct {
	// Desktop: publishing.
	Enabled       bool   `json:"enabled"`
	Path          string `json:"path"`
	LastPublished string `json:"lastPublished"`

	// Server: loading.
	Root        string `json:"root"`
	Loaded      bool   `json:"loaded"`
	SourceRoot  string `json:"sourceRoot"`
	PublishedAt string `json:"publishedAt"`
	LoadedAt    string `json:"loadedAt"`
	Frames      int    `json:"frames"`

	LastError string `json:"lastError"`
}

// formatTime renders t as RFC 3339, or "" for the zero time.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
