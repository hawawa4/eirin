// ── Project frame helpers ──────────────────────────────────────────────────

import type * as app from "$models/app";
import { formatExpTime, plural } from "../utils";

/** Frame types that have a folder in a Siril project (lights/ darks/ flats/ biases/). */
export type ProjectFrameType = "light" | "dark" | "flat" | "bias";

export const PROJECT_FRAME_TYPES: readonly ProjectFrameType[] = ["light", "dark", "flat", "bias"];

/** Whether a frame of this type has a home in a Siril project (see internal/projectfs). */
export function isProjectFrameType(t: string): t is ProjectFrameType {
  return (PROJECT_FRAME_TYPES as readonly string[]).includes(t);
}

export const PROJECT_TYPE_LABEL: Record<ProjectFrameType, { one: string; many: string }> = {
  light: { one: "light", many: "Lights" },
  dark: { one: "dark", many: "Darks" },
  flat: { one: "flat", many: "Flats" },
  bias: { one: "bias", many: "Biases" },
};

/** The least a frame needs to be sorted into project sections. */
export interface TypedFrame {
  nasPath: string;
  frameType: string;
}

export interface FrameSection<F extends TypedFrame = app.LibraryFrame> {
  type: string;
  label: string;
  frames: F[];
}

/** Splits frames into the four project sections, plus "Other" for anything unexpected. */
export function sectionsByType<F extends TypedFrame>(frames: F[]): FrameSection<F>[] {
  const byType = new Map<string, F[]>();
  for (const t of PROJECT_FRAME_TYPES) byType.set(t, []);
  const other: F[] = [];
  for (const f of frames) {
    const list = byType.get(f.frameType);
    if (list) list.push(f);
    else other.push(f);
  }
  const sections: FrameSection<F>[] = PROJECT_FRAME_TYPES.map((t) => ({
    type: t,
    label: PROJECT_TYPE_LABEL[t].many,
    frames: byType.get(t) ?? [],
  }));
  if (other.length) sections.push({ type: "other", label: "Other", frames: other });
  return sections;
}

/** Frame count per project type, in folder order, e.g. "120 lights · 30 darks". */
export function describeTypeCounts(frames: TypedFrame[]): string {
  return sectionsByType(frames)
    .filter((s) => s.frames.length > 0 && s.type !== "other")
    .map((s) => {
      const t = s.type as ProjectFrameType;
      return plural(
        s.frames.length,
        PROJECT_TYPE_LABEL[t].one,
        PROJECT_TYPE_LABEL[t].many.toLowerCase(),
      );
    })
    .join(" · ");
}

/**
 * A default project name from what the lights have in common: the object, plus
 * the telescope when there's a single one ("M 31 Seestar S50"). Empty when the
 * lights span several objects.
 */
export function suggestProjectName(frames: app.LibraryFrame[]): string {
  const lights = frames.filter((f) => f.frameType === "light");
  const only = (vals: string[]) => {
    const set = new Set(vals.map((v) => v.trim()).filter(Boolean));
    return set.size === 1 ? [...set][0] : "";
  };
  const object = only(lights.map((f) => f.object));
  if (!object) return "";
  const telescope = only(lights.map((f) => f.telescope));
  return telescope ? `${object} ${telescope}` : object;
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

/**
 * Makes a single path segment safe to use as a folder name on any OS: path
 * separators and characters invalid on Windows (\ / : * ? " < > |) become "_",
 * control characters are dropped, and trailing dots/spaces are trimmed.
 */
export function safeSubfolderName(name: string): string {
  return (
    name
      // eslint-disable-next-line no-control-regex
      .replace(/[\u0000-\u001f]/g, "")
      .replace(/[\\/:*?"<>|]/g, "_")
      .trim()
      .replace(/[. ]+$/, "")
  );
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
