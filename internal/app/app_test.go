package app

import "testing"

func TestNewApp(t *testing.T) {
	a := NewApp()
	if a == nil {
		t.Fatal("NewApp() returned nil")
	}
	if a.store != nil {
		t.Error("prefs should be nil before Startup")
	}
	if a.wails != nil {
		t.Error("wails should be nil before ServiceStartup")
	}
	if a.server != nil {
		t.Error("server should be nil before Startup")
	}
}
