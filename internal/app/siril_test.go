package app

import (
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func TestSirilExecutableDefault(t *testing.T) {
	a := newTestApp(t)
	// No configured path → falls back to "siril".
	if got := a.sirilExecutable(); got != "siril" {
		t.Errorf("sirilExecutable() = %q, want siril", got)
	}
}

func TestSirilExecutableConfigured(t *testing.T) {
	a := newTestApp(t)
	a.store.Set(store.KeySirilPath, "/opt/siril/bin/siril")

	if got := a.sirilExecutable(); got != "/opt/siril/bin/siril" {
		t.Errorf("sirilExecutable() = %q, want /opt/siril/bin/siril", got)
	}
}

func TestSirilExecutableConfiguredOverridesDefault(t *testing.T) {
	a := newTestApp(t)
	a.store.Set(store.KeySirilPath, "/custom/siril")

	got := a.sirilExecutable()
	if got == "siril" {
		t.Error("configured path should override default 'siril'")
	}
	if got != "/custom/siril" {
		t.Errorf("sirilExecutable() = %q, want /custom/siril", got)
	}
}
