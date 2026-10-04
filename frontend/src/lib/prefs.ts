// ── Preference persistence ──────────────────────────────────────────────────
// The desktop app stores preferences in its database (SetPref). The read-only
// server viewer can't write, and is shared by everyone who opens it, so there
// each browser keeps its own preferences in localStorage instead.

import { LoadPrefs, SetPref } from "$app";
import type * as store from "$models/store";

const LOCAL_PREFIX = "eirin.pref.";

// Preferences a viewer can change, and how to apply a stored string value.
const LOCAL_KEYS: Record<string, (p: store.Prefs, v: string) => void> = {
  theme: (p, v) => (p.theme = v),
  ui_state: (p, v) => (p.uiState = v),
  stretch_enabled: (p, v) => (p.stretchEnabled = v === "true"),
  stretch_level: (p, v) => {
    const n = Number(v);
    if (Number.isInteger(n)) p.stretchLevel = n;
  },
  basic_collapsed: (p, v) => (p.basicCollapsed = v === "true"),
  advanced_collapsed: (p, v) => (p.advancedCollapsed = v === "true"),
  column_config: (p, v) => (p.columnConfig = v),
  library_column_config: (p, v) => (p.libraryColumnConfig = v),
};

let local = false;

/** Switches persistence to this browser's localStorage (read-only server viewer). */
export function usePrefsLocally(on: boolean): void {
  local = on;
}

/** Persists one preference. Fire-and-forget: failures only lose the value. */
export function savePref(key: string, value: string): void {
  if (!local) {
    void SetPref(key, value);
    return;
  }
  try {
    localStorage.setItem(LOCAL_PREFIX + key, value);
  } catch {
    /* storage unavailable (private window, blocked site data) */
  }
}

/** Loads preferences, with this browser's own values on top when local. */
export async function loadPrefs(): Promise<store.Prefs> {
  const p = await LoadPrefs();
  if (!local) return p;
  for (const [key, apply] of Object.entries(LOCAL_KEYS)) {
    try {
      const v = localStorage.getItem(LOCAL_PREFIX + key);
      if (v !== null) apply(p, v);
    } catch {
      /* storage unavailable */
    }
  }
  return p;
}
