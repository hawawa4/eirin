// ── Blink sets ──────────────────────────────────────────────────────────────
// Blink shows the checked frames when 2+ are checked; otherwise it blinks the
// whole object of the frame being looked at, within the current view.

import type * as app from "$models/app";
import { plural } from "../utils";

export interface BlinkPlan {
  frames: app.LibraryFrame[];
  /** Frame to start on. */
  start: string | null;
  /** Describes the set for buttons and tooltips, e.g. "12 checked frames". */
  label: string;
}

const norm = (s: string) => s.trim().toLowerCase();

/** Every frame in `ordered` with the same object and frame type as `anchor`, in order. */
export function objectBlinkSet(
  ordered: app.LibraryFrame[],
  anchor: app.LibraryFrame,
): app.LibraryFrame[] {
  const obj = norm(anchor.object);
  return ordered.filter((f) => f.frameType === anchor.frameType && norm(f.object) === obj);
}

const TYPE_PLURAL: Record<string, string> = {
  light: "lights",
  dark: "darks",
  flat: "flats",
  bias: "biases",
  stacked: "stacks",
  processed: "processed images",
  image: "images",
};

/** "M 31 lights", "darks" (no object), … */
export function objectSetName(anchor: app.LibraryFrame): string {
  const kind = TYPE_PLURAL[anchor.frameType] ?? "frames";
  return anchor.object.trim() ? `${anchor.object.trim()} ${kind}` : kind;
}

/**
 * What Blink would show. `ordered` is the current view in display order;
 * `checked` the checked frames (display order first); `anchor` the previewed
 * frame, or the single checked one.
 */
export function planBlink(
  ordered: app.LibraryFrame[],
  checked: app.LibraryFrame[],
  anchor: app.LibraryFrame | undefined,
): BlinkPlan | null {
  if (checked.length >= 2) {
    const start =
      anchor && checked.some((f) => f.nasPath === anchor.nasPath) ? anchor.nasPath : null;
    return { frames: checked, start, label: plural(checked.length, "checked frame") };
  }
  if (!anchor) return null;
  const set = objectBlinkSet(ordered, anchor);
  if (set.length < 2) return null;
  return {
    frames: set,
    start: anchor.nasPath,
    label: `all ${set.length} ${objectSetName(anchor)}`,
  };
}
