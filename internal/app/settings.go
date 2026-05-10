package app

import "fmt"

// AppInfo contains read-only runtime information surfaced in the Settings view.
type AppInfo struct {
	DBPath     string `json:"dbPath"`
	ServerPort int    `json:"serverPort"`
	ServerURL  string `json:"serverUrl"`
	PortSource string `json:"portSource"` // "EIRIN_PORT env var" or "default"
}

// GetAppInfo returns static runtime information about the application.
func (a *App) GetAppInfo() AppInfo {
	port := serverPort()
	source := "default"
	if p := getEnv("EIRIN_PORT"); p != "" {
		source = "EIRIN_PORT env var"
	}
	return AppInfo{
		DBPath:     a.prefs.DBPath(),
		ServerPort: port,
		ServerURL:  fmt.Sprintf("http://localhost:%d", port),
		PortSource: source,
	}
}
