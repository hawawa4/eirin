package app

import (
	"fmt"

	"github.com/TaruDesigns/eirin/internal/server"
)

// AppInfo contains read-only runtime information surfaced in the Settings view.
type AppInfo struct {
	DBPath     string `json:"dbPath"`
	ServerPort int    `json:"serverPort"`
	ServerURL  string `json:"serverUrl"`
	PortSource string `json:"portSource"` // "EIRIN_PORT env var" or "default"
}

// GetAppInfo returns static runtime information about the application.
func (a *App) GetAppInfo() AppInfo {
	port := server.Port()
	source := "default"
	if p := server.GetEnv("EIRIN_PORT"); p != "" {
		source = "EIRIN_PORT env var"
	}
	return AppInfo{
		DBPath:     a.store.DBPath(),
		ServerPort: port,
		ServerURL:  fmt.Sprintf("http://localhost:%d", port),
		PortSource: source,
	}
}
