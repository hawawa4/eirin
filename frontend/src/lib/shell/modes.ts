// ── App modes (top-level tabs) ──────────────────────────────────────────────
// Single source of truth for tab order, grouping and availability, shared by the
// header's ModeSelector and App's view mounting.

import type { AppMode } from "../types";

/**
 * "visualize" tabs are for looking at the library (sky, slideshow); "manage" tabs
 * are for working on it (import, cull, build projects). Settings sits on its own.
 */
export type ModeGroup = "visualize" | "manage" | "settings";

export interface ModeDef {
  value: AppMode;
  label: string;
  icon: string;
  group: ModeGroup;
  /** One-line explanation shown in the tab's tooltip. */
  hint: string;
  /** Needs a root folder to be useful (App shows the "no folder" empty state otherwise). */
  requiresRoot: boolean;
  /** Shown in the read-only server viewer (which only has these tabs). */
  inViewer?: boolean;
}

export const MODES: readonly ModeDef[] = [
  {
    value: "atlas",
    label: "Sky Atlas",
    icon: "✦",
    group: "visualize",
    hint: "Your images placed on the sky",
    requiresRoot: true,
    inViewer: true,
  },
  {
    value: "slideshow",
    label: "Slideshow",
    icon: "▶",
    group: "visualize",
    hint: "Full-screen show of your stacked and processed images",
    requiresRoot: true,
    inViewer: true,
  },
  {
    value: "import",
    label: "Import",
    icon: "⇪",
    group: "manage",
    hint: "Copy new data from the Seestar into the library",
    requiresRoot: true,
  },
  {
    value: "library",
    label: "Library",
    icon: "◈",
    group: "manage",
    hint: "Every frame in the library: filter, cull, blink, build projects",
    requiresRoot: true,
    inViewer: true,
  },
  {
    value: "projects",
    label: "Projects",
    icon: "◧",
    group: "manage",
    hint: "Siril projects and their outputs",
    requiresRoot: true,
  },
  {
    value: "coverage",
    label: "Coverage",
    icon: "▣",
    group: "manage",
    hint: "Light frames grouped by framing on the sky, to pick data for projects",
    requiresRoot: true,
  },
  {
    value: "storage",
    label: "Storage",
    icon: "◉",
    group: "manage",
    hint: "Disk usage by object and night; free space by deleting rejects",
    requiresRoot: true,
  },
  {
    value: "settings",
    label: "Settings",
    icon: "⚙",
    group: "settings",
    hint: "Library folder, Siril, appearance and updates",
    requiresRoot: false,
    inViewer: true,
  },
];

/** The tabs to show: all of them, or just the viewer's in read-only mode. */
export function visibleModes(readOnly: boolean): readonly ModeDef[] {
  return readOnly ? MODES.filter((m) => m.inViewer) : MODES;
}

/** The app opens on the Sky Atlas. */
export const DEFAULT_MODE: AppMode = "atlas";

/** Why a mode can't be entered right now, or null when it can. */
export function modeUnavailableReason(m: ModeDef, rootFolder: string): string | null {
  if (m.requiresRoot && !rootFolder) return "Select a root folder first";
  return null;
}

/**
 * The tab a Ctrl+digit shortcut switches to: digits count the visible tabs in
 * header order (settings excluded), so the numbers match what's on screen.
 */
export function modeForDigit(readOnly: boolean, digit: number): ModeDef | null {
  const tabs = visibleModes(readOnly).filter((m) => m.group !== "settings");
  return tabs[digit - 1] ?? null;
}
