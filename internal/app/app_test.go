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
	if a.ctx != nil {
		t.Error("ctx should be nil before Startup")
	}
	if a.server != nil {
		t.Error("server should be nil before Startup")
	}
}
