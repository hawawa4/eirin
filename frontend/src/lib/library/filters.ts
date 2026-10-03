// ── Library filtering ───────────────────────────────────────────────────────
// Pure helpers for the Library table's search box and per-column filters.

import type * as app from "$models/app";
import type { ColFilter, FrameType } from "../types";
import { getFrameNumVal, getFrameTextVal } from "../utils";

export type ColFilters = Record<string, ColFilter>;

export const TEXT_FILTER_COLS: ReadonlySet<string> = new Set([
  "name",
  "object",
  "filter",
  "telescope",
  "instrument",
  "dateObs",
]);

/** Text columns that also offer a list of the values in the library. */
export const VALUE_PICK_COLS: ReadonlySet<string> = new Set(["object", "filter", "telescope"]);

export const NUMERIC_FILTER_COLS: ReadonlySet<string> = new Set([
  "expTime",
  "size",
  "gain",
  "ccdTemp",
  "fwhm",
  "starCount",
  "background",
  "noise",
  "snr",
]);

// Columns whose zero value is rendered as "—" (no data) rather than a real 0.
const ZERO_IS_EMPTY: ReadonlySet<string> = new Set([
  "expTime",
  "gain",
  "ccdTemp",
  "fwhm",
  "starCount",
  "background",
  "noise",
  "snr",
]);

export function isColFilterActive(filters: ColFilters, colId: string): boolean {
  const cf = filters[colId];
  if (!cf) return false;
  if (colId === "frameType") return (cf.types?.length ?? 0) > 0;
  if (TEXT_FILTER_COLS.has(colId)) return !!cf.text;
  if (NUMERIC_FILTER_COLS.has(colId)) return cf.numOp != null && cf.numVal != null;
  return false;
}

export function anyColFilterActive(filters: ColFilters): boolean {
  return Object.keys(filters).some((k) => isColFilterActive(filters, k));
}

/** Numeric cell value, or null when the cell shows "—" (missing / not analyzed). */
function numericValue(f: app.LibraryFrame, colId: string): number | null {
  const v = getFrameNumVal(f, colId);
  if (v == null || Number.isNaN(v)) return null;
  if (v === 0 && ZERO_IS_EMPTY.has(colId)) return null;
  return v;
}

/** True if `f` passes the search box (case-insensitive substring on name/object/filter). */
export function matchesSearch(f: app.LibraryFrame, query: string): boolean {
  if (!query) return true;
  const q = query.toLowerCase();
  return (
    f.fileName.toLowerCase().includes(q) ||
    f.object.toLowerCase().includes(q) ||
    f.filter.toLowerCase().includes(q)
  );
}

/** True if `f` passes every active column filter. */
export function matchesColFilters(f: app.LibraryFrame, filters: ColFilters): boolean {
  for (const [colId, cf] of Object.entries(filters)) {
    if (colId === "frameType") {
      if (cf.types && cf.types.length > 0 && !cf.types.includes(f.frameType as FrameType))
        return false;
    } else if (TEXT_FILTER_COLS.has(colId) && cf.text) {
      const val = getFrameTextVal(f, colId).toLowerCase();
      const want = cf.text.toLowerCase();
      if (cf.exact ? val.trim() !== want : !val.includes(want)) return false;
    } else if (NUMERIC_FILTER_COLS.has(colId) && cf.numOp && cf.numVal != null) {
      // While a numeric filter is active, frames without a value never match.
      const val = numericValue(f, colId);
      if (val === null) return false;
      if (cf.numOp === "<" && val >= cf.numVal) return false;
      if (cf.numOp === ">" && val <= cf.numVal) return false;
    }
  }
  return true;
}

export interface ValueCount {
  value: string;
  count: number;
}

/**
 * Distinct non-empty values of a text column, with frame counts, among the
 * frames that pass the search and every *other* column filter, so picking an
 * object narrows the telescope list to what that object was shot with.
 */
export function distinctValues(
  frames: app.LibraryFrame[],
  colId: string,
  search: string,
  filters: ColFilters,
): ValueCount[] {
  const others: ColFilters = { ...filters };
  delete others[colId];
  const counts = new Map<string, number>();
  for (const f of frames) {
    if (!matchesSearch(f, search) || !matchesColFilters(f, others)) continue;
    const v = getFrameTextVal(f, colId).trim();
    if (v) counts.set(v, (counts.get(v) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([value, count]) => ({ value, count }))
    .sort((a, b) => a.value.localeCompare(b.value, undefined, { numeric: true }));
}
