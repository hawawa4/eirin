package app

import (
	"fmt"
	"testing"

	"github.com/TaruDesigns/eirin/internal/server"
)

func TestGetAppInfoDefaults(t *testing.T) {
	a := newTestApp(t)
	t.Setenv("EIRIN_PORT", "")

	info := a.GetAppInfo()

	if info.ServerPort != server.DefaultPort {
		t.Errorf("ServerPort = %d, want default %d", info.ServerPort, server.DefaultPort)
	}
	if info.PortSource != "default" {
		t.Errorf("PortSource = %q, want default", info.PortSource)
	}
	if info.DBPath == "" {
		t.Error("DBPath should not be empty")
	}
	wantURL := fmt.Sprintf("http://localhost:%d", server.DefaultPort)
	if info.ServerURL != wantURL {
		t.Errorf("ServerURL = %q, want %q", info.ServerURL, wantURL)
	}
}

func TestGetAppInfoCustomPort(t *testing.T) {
	a := newTestApp(t)
	t.Setenv("EIRIN_PORT", "8888")

	info := a.GetAppInfo()

	if info.ServerPort != 8888 {
		t.Errorf("ServerPort = %d, want 8888", info.ServerPort)
	}
	if info.PortSource != "EIRIN_PORT env var" {
		t.Errorf("PortSource = %q, want EIRIN_PORT env var", info.PortSource)
	}
	if info.ServerURL != "http://localhost:8888" {
		t.Errorf("ServerURL = %q, want http://localhost:8888", info.ServerURL)
	}
}

func TestGetAppInfoDBPathMatchesStore(t *testing.T) {
	a := newTestApp(t)
	info := a.GetAppInfo()
	if info.DBPath != a.store.DBPath() {
		t.Errorf("AppInfo.DBPath = %q, store.DBPath() = %q — should match",
			info.DBPath, a.store.DBPath())
	}
}
