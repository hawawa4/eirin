// ── Library grouping & sorting ──────────────────────────────────────────────

import type * as app from "$models/app";
import type { LibraryGroup, LibraryGroupBy } from "../types";
import { FRAME_TYPE_META } from "../types";
import { getFrameSortVal } from "../utils";
import type { LibrarySort } from "../uiState.svelte";

export interface FrameTypeMeta {
  label: string;
  short: string;
  color: string;
  bg: string;
}

export interface TypeCount {
  type: string;
  count: number;
  meta: FrameTypeMeta;
}

export interface LibGroup extends LibraryGroup {
  /** Per-type counts, in TYPE_ORDER (computed once per grouping pass). */
  breakdown: TypeCount[];
  hasQuality: boolean;
  unanalyzedCount: number;
}

export const GROUP_BY_OPTIONS: { value: LibraryGroupBy; label: string }[] = [
  { value: "object", label: "Object" },
  { value: "date", label: "Date" },
  { value: "filter", label: "Filter" },
  { value: "frameType", label: "Type" },
];

const TYPE_ORDER = ["light", "stacked", "processed", "image", "flat", "dark", "bias"];

export function toGroupBy(v: string): LibraryGroupBy {
  return GROUP_BY_OPTIONS.some((o) => o.value === v) ? (v as LibraryGroupBy) : "object";
}

export function frameTypeMeta(type: string): FrameTypeMeta {
  return (
    FRAME_TYPE_META[type] ?? {
      label: type,
      short: type.toUpperCase(),
      color: "#9ca3af",
      bg: "#1f2937",
    }
  );
}

export function groupKeyFor(f: app.LibraryFrame, groupBy: LibraryGroupBy): string {
  switch (groupBy) {
    case "object":
      return f.object || "(unknown object)";
    case "date":
      return f.dateObs ? f.dateObs.slice(0, 10) : "(no date)";
    case "filter":
      return f.filter || "(no filter)";
    case "frameType":
      return f.frameType || "stacked";
  }
}

function labelForKey(key: string, groupBy: LibraryGroupBy): string {
  if (groupBy === "frameType") return FRAME_TYPE_META[key]?.label ?? key;
  return key;
}

function typeRank(t: string): number {
  const i = TYPE_ORDER.indexOf(t);
  return i === -1 ? 99 : i;
}

function breakdownOf(frames: app.LibraryFrame[]): TypeCount[] {
  const counts = new Map<string, number>();
  for (const f of frames) counts.set(f.frameType, (counts.get(f.frameType) ?? 0) + 1);
  return [...counts.entries()]
    .sort((a, b) => typeRank(a[0]) - typeRank(b[0]))
    .map(([type, count]) => ({ type, count, meta: frameTypeMeta(type) }));
}

/** Buckets `items` (already sorted) into groups ordered by key. */
export function buildGroups(items: app.LibraryFrame[], groupBy: LibraryGroupBy): LibGroup[] {
  const map = new Map<string, app.LibraryFrame[]>();
  for (const f of items) {
    const key = groupKeyFor(f, groupBy);
    const bucket = map.get(key);
    if (bucket) bucket.push(f);
    else map.set(key, [f]);
  }
  const result: LibGroup[] = [];
  for (const [key, frames] of map) {
    let hasQuality = false;
    let unanalyzedCount = 0;
    for (const f of frames) {
      if (f.qualityAnalyzed) hasQuality = true;
      else unanalyzedCount++;
    }
    result.push({
      key,
      label: labelForKey(key, groupBy),
      frames,
      breakdown: breakdownOf(frames),
      hasQuality,
      unanalyzedCount,
    });
  }
  result.sort((a, b) => a.key.localeCompare(b.key));
  return result;
}

/** path → frame lookup. */
export function indexByPath(frames: app.LibraryFrame[]): Map<string, app.LibraryFrame> {
  return new Map(frames.map((f) => [f.nasPath, f]));
}

/** path → position in `rows`. */
export function positionsByPath(rows: app.LibraryFrame[]): Map<string, number> {
  return new Map(rows.map((f, i) => [f.nasPath, i]));
}

/**
 * The row the cursor should land on after `removed` rows leave `rows`: the current
 * row if it stays, else the next surviving row below it, else the nearest above.
 */
export function cursorAfterRemoval(
  rows: app.LibraryFrame[],
  current: string | null,
  removed: ReadonlySet<string>,
): string | null {
  if (!current) return null;
  if (!removed.has(current)) return current;
  const i = rows.findIndex((f) => f.nasPath === current);
  if (i === -1) return null;
  for (let j = i + 1; j < rows.length; j++)
    if (!removed.has(rows[j].nasPath)) return rows[j].nasPath;
  for (let j = i - 1; j >= 0; j--) if (!removed.has(rows[j].nasPath)) return rows[j].nasPath;
  return null;
}

export function sortFrames(
  frames: app.LibraryFrame[],
  sort: LibrarySort | null,
): app.LibraryFrame[] {
  if (!sort) return frames;
  const { col, dir } = sort;
  const mul = dir === "asc" ? 1 : -1;
  return [...frames].sort((a, b) => {
    const av = getFrameSortVal(a, col);
    const bv = getFrameSortVal(b, col);
    if (av < bv) return -1 * mul;
    if (av > bv) return 1 * mul;
    return 0;
  });
}
