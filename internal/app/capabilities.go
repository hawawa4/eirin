package app

// Capabilities describes which features are available in the current build.
// Desktop builds (default) expose everything; headless builds (-tags server,
// used for the Docker deployment) are a read-only viewer of final images fed
// by the desktop's library snapshot.
type Capabilities struct {
	DesktopMode bool `json:"desktopMode"`
	// ReadOnly: every write method returns errReadOnly (see requireWritable).
	ReadOnly bool `json:"readOnly"`
}

func currentCapabilities() Capabilities {
	return Capabilities{DesktopMode: desktopModeEnabled, ReadOnly: !desktopModeEnabled}
}
