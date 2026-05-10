package app

import (
	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// LoadPrefs returns all persisted preferences, with defaults for missing keys.
func (a *App) LoadPrefs() prefs.Prefs {
	if a.prefs == nil {
		return prefs.DefaultPrefs()
	}
	return a.prefs.Load()
}

// SetPref persists a single preference by key. Both key and value are strings;
// booleans are "true"/"false" and integers are their decimal string form.
func (a *App) SetPref(key, value string) {
	if a.prefs == nil {
		return
	}
	if err := a.prefs.Set(key, value); err != nil {
		runtime.LogErrorf(a.ctx, "prefs: set %q=%q: %v", key, value, err)
	}
}
