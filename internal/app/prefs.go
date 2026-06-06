package app

import (
	"log/slog"

	"github.com/TaruDesigns/eirin/internal/store"
)

// LoadPrefs returns all persisted preferences, with defaults for missing keys.
func (a *App) LoadPrefs() store.Prefs {
	if a.store == nil {
		return store.DefaultPrefs()
	}
	return a.store.Load()
}

// SetPref persists a single preference by key. Both key and value are strings;
// booleans are "true"/"false" and integers are their decimal string form.
func (a *App) SetPref(key, value string) {
	if a.store == nil {
		return
	}
	if err := a.store.Set(key, value); err != nil {
		slog.Error("prefs: set", "key", key, "value", value, "err", err)
	}
}
