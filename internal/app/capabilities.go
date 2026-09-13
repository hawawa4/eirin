package app

// Capabilities describes which features are available in the current build.
// Desktop builds (default) expose everything; headless builds (-tags server,
// used for the Docker/NAS-mini-PC deployment) are read-only visualization
// only — no import, no Siril processing, no project management.
type Capabilities struct {
	DesktopMode bool `json:"desktopMode"`
}

func currentCapabilities() Capabilities {
	return Capabilities{DesktopMode: desktopModeEnabled}
}
