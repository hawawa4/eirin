// ── Persisted preview preferences ───────────────────────────────────────────
// Shared by every PreviewPane instance (Browse and Library), persisted as
// individual prefs for backwards compatibility with existing settings.

import { savePref } from "./prefs";
import type * as store from "$models/store";

const PREF_STRETCH_ENABLED = "stretch_enabled";
const PREF_STRETCH_LEVEL = "stretch_level";
const PREF_BASIC_COLLAPSED = "basic_collapsed";
const PREF_ADVANCED_COLLAPSED = "advanced_collapsed";

export const previewPrefs = $state({
  stretchEnabled: true,
  stretchLevel: 2,
  basicCollapsed: false,
  advancedCollapsed: true,
});

let loaded = false;

/** Applies persisted values and starts saving changes. Call once after LoadPrefs. */
export function loadPreviewPrefs(p: store.Prefs): void {
  previewPrefs.stretchEnabled = p.stretchEnabled;
  previewPrefs.stretchLevel = p.stretchLevel;
  previewPrefs.basicCollapsed = p.basicCollapsed;
  previewPrefs.advancedCollapsed = p.advancedCollapsed;
  if (loaded) return;
  loaded = true;

  $effect.root(() => {
    $effect(() => {
      savePref(PREF_STRETCH_ENABLED, String(previewPrefs.stretchEnabled));
    });
    $effect(() => {
      savePref(PREF_STRETCH_LEVEL, String(previewPrefs.stretchLevel));
    });
    $effect(() => {
      savePref(PREF_BASIC_COLLAPSED, String(previewPrefs.basicCollapsed));
    });
    $effect(() => {
      savePref(PREF_ADVANCED_COLLAPSED, String(previewPrefs.advancedCollapsed));
    });
  });
}
