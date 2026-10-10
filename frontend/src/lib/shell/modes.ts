// ── App modes (top-level tabs) ──────────────────────────────────────────────
// Single source of truth for tab order and availability, shared by the header's
// ModeSelector and App's view mounting.

import type { AppMode } from "../types";

export interface ModeDef {
  value: AppMode;
  label: string;
  icon: string;
  /** Needs a root folder to be useful (App shows the "no folder" empty state otherwise). */
  requiresRoot: boolean;
  /** Shown in the read-only server viewer (which only has these tabs). */
  inViewer?: boolean;
}

export const MODES: readonly ModeDef[] = [
  { value: "library", label: "Library", icon: "◈", requiresRoot: true, inViewer: true },
  { value: "projects", label: "Projects", icon: "◧", requiresRoot: true },
  { value: "import", label: "Import", icon: "⇪", requiresRoot: true },
  { value: "atlas", label: "Sky Atlas", icon: "✦", requiresRoot: true, inViewer: true },
  { value: "coverage", label: "Coverage", icon: "▣", requiresRoot: true },
  { value: "storage", label: "Storage", icon: "◉", requiresRoot: true },
  { value: "browser", label: "Browse", icon: "⊞", requiresRoot: true },
  { value: "settings", label: "Settings", icon: "⚙", requiresRoot: false, inViewer: true },
];

/** The tabs to show: all of them, or just the viewer's in read-only mode. */
export function visibleModes(readOnly: boolean): readonly ModeDef[] {
  return readOnly ? MODES.filter((m) => m.inViewer) : MODES;
}

export const DEFAULT_MODE: AppMode = "library";

/** Why a mode can't be entered right now, or null when it can. */
export function modeUnavailableReason(m: ModeDef, rootFolder: string): string | null {
  if (m.requiresRoot && !rootFolder) return "Select a root folder first";
  return null;
}
