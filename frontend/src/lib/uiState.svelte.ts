// ── Persisted UI state ──────────────────────────────────────────────────────
// Small layout/view preferences stored as one JSON blob under the "ui_state" pref.

import { savePref } from "./prefs";

export const PREF_UI_STATE = "ui_state";

export type UiScale = 14 | 16 | 18;
export const UI_SCALES: readonly UiScale[] = [14, 16, 18];
export const DEFAULT_UI_SCALE: UiScale = 16;

export interface LibrarySort {
  col: string;
  dir: "asc" | "desc";
}

export interface UiState {
  splitPct: number;
  libraryGroupBy: string;
  librarySort: LibrarySort | null;
  lastProjectId: number | null;
  uiScale: UiScale;
  /** Library: per-column filter row under the table header is shown. */
  libraryFilterRow: boolean;
  /** Sky Atlas: the "how to use it" hint was dismissed. */
  atlasHintDismissed: boolean;
}

export const ui: UiState = $state({
  splitPct: 42,
  libraryGroupBy: "object",
  librarySort: null,
  lastProjectId: null,
  uiScale: DEFAULT_UI_SCALE,
  libraryFilterRow: false,
  atlasHintDismissed: false,
});

const SAVE_DEBOUNCE_MS = 400;
let loaded = false;

function merge(raw: unknown) {
  if (!raw || typeof raw !== "object") return;
  const s = raw as Record<string, unknown>;
  if (typeof s.splitPct === "number" && s.splitPct >= 15 && s.splitPct <= 75) {
    ui.splitPct = s.splitPct;
  }
  if (typeof s.libraryGroupBy === "string") ui.libraryGroupBy = s.libraryGroupBy;
  const sort = s.librarySort as Partial<LibrarySort> | null | undefined;
  if (sort === null) ui.librarySort = null;
  else if (sort && typeof sort.col === "string" && (sort.dir === "asc" || sort.dir === "desc")) {
    ui.librarySort = { col: sort.col, dir: sort.dir };
  }
  if (s.lastProjectId === null || typeof s.lastProjectId === "number") {
    ui.lastProjectId = s.lastProjectId;
  }
  if (UI_SCALES.includes(s.uiScale as UiScale)) ui.uiScale = s.uiScale as UiScale;
  if (typeof s.libraryFilterRow === "boolean") ui.libraryFilterRow = s.libraryFilterRow;
  if (typeof s.atlasHintDismissed === "boolean") ui.atlasHintDismissed = s.atlasHintDismissed;
}

/**
 * Applies the persisted JSON (from Prefs.uiState) and starts the debounced auto-save.
 * Call once after LoadPrefs; later calls only re-merge.
 */
export function loadUiState(json: string | undefined): void {
  if (json) {
    try {
      merge(JSON.parse(json));
    } catch {
      /* keep defaults */
    }
  }
  if (loaded) return;
  loaded = true;

  let lastSaved = JSON.stringify(ui);
  let timer: ReturnType<typeof setTimeout> | undefined;
  $effect.root(() => {
    $effect(() => {
      const snapshot = JSON.stringify(ui);
      // Cancel a pending save first: a change reverted within the debounce
      // window must not persist the stale intermediate state.
      clearTimeout(timer);
      if (snapshot === lastSaved) return;
      timer = setTimeout(() => {
        lastSaved = snapshot;
        savePref(PREF_UI_STATE, snapshot);
      }, SAVE_DEBOUNCE_MS);
    });
  });
}
