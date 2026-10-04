package app

import (
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func TestUIStateRoundTrip(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	if got := a.LoadPrefs().UIState; got != "" {
		t.Errorf("default UIState = %q, want empty", got)
	}
	blob := `{"tab":"projects"}`
	a.SetPref(store.KeyUIState, blob)
	if got := a.LoadPrefs().UIState; got != blob {
		t.Errorf("UIState = %q, want %q", got, blob)
	}
}
