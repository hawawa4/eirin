// ── Project frame helpers ──────────────────────────────────────────────────

import type * as app from "$models/app";
import { formatExpTime } from "../utils";

/** Frame types that have a folder in a Siril project (lights/ darks/ flats/ biases/). */
export type ProjectFrameType = "light" | "dark" | "flat" | "bias";

export const PROJECT_FRAME_TYPES: readonly ProjectFrameType[] = ["light", "dark", "flat", "bias"];

export const PROJECT_TYPE_LABEL: Record<ProjectFrameType, { one: string; many: string }> = {
  light: { one: "light", many: "Lights" },
  dark: { one: "dark", many: "Darks" },
  flat: { one: "flat", many: "Flats" },
  bias: { one: "bias", many: "Biases" },
};

export interface FrameSection {
  type: string;
  label: string;
  frames: app.LibraryFrame[];
}

/** Splits frames into the four project sections, plus "Other" for anything unexpected. */
export function sectionsByType(frames: app.LibraryFrame[]): FrameSection[] {
  const byType = new Map<string, app.LibraryFrame[]>();
  for (const t of PROJECT_FRAME_TYPES) byType.set(t, []);
  const other: app.LibraryFrame[] = [];
  for (const f of frames) {
    const list = byType.get(f.frameType);
    if (list) list.push(f);
    else other.push(f);
  }
  const sections: FrameSection[] = PROJECT_FRAME_TYPES.map((t) => ({
    type: t,
    label: PROJECT_TYPE_LABEL[t].many,
    frames: byType.get(t) ?? [],
  }));
  if (other.length) sections.push({ type: "other", label: "Other", frames: other });
  return sections;
}

export function plural(n: number, one: string, many = one + "s"): string {
  return `${n} ${n === 1 ? one : many}`;
}

/** One-line summary of an AddFramesToProjectDetailed result. */
export function describeAddResult(r: app.AddFramesResult): string {
  const parts: string[] = [];
  if (r.added > 0) parts.push(`Added ${plural(r.added, "frame")}`);
  else parts.push("No new frames added");
  if (r.alreadyPresent > 0) parts.push(`${r.alreadyPresent} already in the project`);
  const skipped = r.skipped?.length ?? 0;
  if (skipped > 0) parts.push(`${skipped} skipped (only light, dark, flat and bias frames fit)`);
  return parts.join(" · ");
}

// ── Picker grouping ──────────────────────────────────────────────────────────
// Lights are grouped by object; calibration frames by the properties they must
// match: darks by exposure + gain, flats by filter, biases by gain.

export function calibrationGroupKey(f: app.LibraryFrame): string {
  switch (f.frameType) {
    case "dark":
      return `${formatExpTime(f.expTime)} · gain ${f.gain || "—"}`;
    case "flat":
      return f.filter ? `Filter ${f.filter}` : "No filter";
    case "bias":
      return `Gain ${f.gain || "—"}`;
    default:
      return f.object || "(no object)";
  }
}

export interface PickerGroup {
  key: string;
  label: string;
  /** Number of frames, when known up front. */
  count?: number;
}

export function groupCalibration(frames: app.LibraryFrame[]): PickerGroup[] {
  const counts = new Map<string, number>();
  for (const f of frames) {
    const k = calibrationGroupKey(f);
    counts.set(k, (counts.get(k) ?? 0) + 1);
  }
  return [...counts.entries()]
    .sort((a, b) => a[0].localeCompare(b[0], undefined, { numeric: true }))
    .map(([key, count]) => ({ key, label: key, count }));
}

/** Mirrors the backend's sanitizeFolderName so the create dialog can preview the folder. */
export function sanitizeFolderName(name: string): string {
  const safe = name
    .trim()
    .replace(/[^\w-]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return safe || "project";
}

/** Joins a folder and a relative subpath with "/" (the paths come from the backend as-is). */
export function joinPath(base: string, sub: string): string {
  const clean = sub.trim().replace(/^[\\/]+|[\\/]+$/g, "");
  if (!clean) return base;
  const sep = base.includes("\\") && !base.includes("/") ? "\\" : "/";
  return base.replace(/[\\/]+$/, "") + sep + clean;
}

export function formatShortDate(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}
