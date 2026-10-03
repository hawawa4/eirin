// ── Library table virtualization ────────────────────────────────────────────
// The table body is a flat list of group headers and frame rows; only the slice
// that intersects the viewport is rendered, padded by spacer rows above/below.

import type * as app from "$models/app";
import type { LibGroup } from "./groups";

export type TableItem =
  | { kind: "group"; key: string; group: LibGroup }
  | { kind: "frame"; key: string; frame: app.LibraryFrame };

/** Flattens groups into render order; frames only for expanded groups. */
export function flattenGroups(
  groups: LibGroup[],
  isExpanded: (key: string) => boolean,
): TableItem[] {
  const items: TableItem[] = [];
  for (const group of groups) {
    items.push({ kind: "group", key: `\u0000group:${group.key}`, group });
    if (!isExpanded(group.key)) continue;
    for (const frame of group.frames) items.push({ kind: "frame", key: frame.nasPath, frame });
  }
  return items;
}

/** offsets[i] is the top of item i; offsets[items.length] is the total height. */
export function itemOffsets(items: TableItem[], rowH: number, groupH: number): Float64Array {
  const offsets = new Float64Array(items.length + 1);
  for (let i = 0; i < items.length; i++) {
    offsets[i + 1] = offsets[i] + (items[i].kind === "group" ? groupH : rowH);
  }
  return offsets;
}

/** Index of the item containing body-relative `y` (clamped to valid items). */
export function itemAt(offsets: Float64Array, y: number): number {
  const n = offsets.length - 1;
  if (n <= 0) return 0;
  let lo = 0;
  let hi = n - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (offsets[mid] <= y) lo = mid;
    else hi = mid - 1;
  }
  return lo;
}

/** [start, end) item range covering body-relative [top, bottom]. */
export function visibleRange(
  offsets: Float64Array,
  top: number,
  bottom: number,
): { start: number; end: number } {
  const n = offsets.length - 1;
  if (n <= 0) return { start: 0, end: 0 };
  return {
    start: itemAt(offsets, Math.max(0, top)),
    end: Math.min(n, itemAt(offsets, bottom) + 1),
  };
}
