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
  /** Desktop-only (hidden behind a disabled tab in headless server mode). */
  requiresDesktop?: boolean;
}

export const MODES: readonly ModeDef[] = [
  { value: "library", label: "Library", icon: "◈", requiresRoot: true },
  { value: "projects", label: "Projects", icon: "◧", requiresRoot: true, requiresDesktop: true },
  { value: "import", label: "Import", icon: "⇪", requiresRoot: true, requiresDesktop: true },
  { value: "atlas", label: "Sky Atlas", icon: "✦", requiresRoot: true },
  { value: "storage", label: "Storage", icon: "◉", requiresRoot: true },
  { value: "browser", label: "Browse", icon: "⊞", requiresRoot: true },
  { value: "settings", label: "Settings", icon: "⚙", requiresRoot: false },
];

export const DEFAULT_MODE: AppMode = "library";

/** Why a mode can't be entered right now, or null when it can. */
export function modeUnavailableReason(
  m: ModeDef,
  rootFolder: string,
  desktopMode: boolean,
): string | null {
  if (m.requiresDesktop && !desktopMode) return "Not available in server mode";
  if (m.requiresRoot && !rootFolder) return "Select a root folder first";
  return null;
}
