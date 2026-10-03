// ── Per-frame orientation fix ───────────────────────────────────────────────
// Escape hatch for frames whose stored rotation doesn't match the image (e.g.
// a plate solver reporting the angle with the opposite sign). Applied to both
// the footprint outline and the preview, which share one transform.

import type * as app from "$models/app";

export interface OrientationFix {
  /** Extra rotation in degrees: -90 | 0 | 90 | 180. */
  offset: number;
  /** Use −rotation instead of rotation. */
  mirror: boolean;
}

export const NO_FIX: OrientationFix = { offset: 0, mirror: false };

export function effectiveRotation(
  entry: app.AtlasIndexEntry,
  fix: OrientationFix | undefined,
): number {
  if (!fix) return entry.rotation;
  return (fix.mirror ? -entry.rotation : entry.rotation) + fix.offset;
}
