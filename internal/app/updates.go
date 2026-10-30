package app

// UpdateInfo is the result of checking GitHub for a newer release.
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Notes     string `json:"notes"`
	PageURL   string `json:"pageUrl"`
	// Kind is how this copy was installed: "appimage", "deb", "binary", or
	// "unsupported". It decides how InstallUpdate applies the update.
	Kind string `json:"kind"`
	// CanInstall is whether InstallUpdate can do the update from inside the
	// app; Reason says why not when it can't (the release page is then the
	// way to update).
	CanInstall bool   `json:"canInstall"`
	Reason     string `json:"reason"`
}
