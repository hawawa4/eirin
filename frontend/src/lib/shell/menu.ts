// ── Action menu items ───────────────────────────────────────────────────────
// Shared item type for ActionMenu.svelte, plus the standard entries for a
// file-like thing (an image on the atlas, a slideshow picture, an import row)
// so every view offers the same actions in the same order.

import { copyPath } from "../clipboard";
import { revealPath } from "./fileActions";

export type ActionMenuItem =
  | {
      label: string;
      run: () => void;
      /** Keyboard shortcut shown on the right, e.g. "x". */
      hint?: string;
      danger?: boolean;
      /** Accent-coloured (the primary thing to do here). */
      primary?: boolean;
      /** Disabled with this explanation as the tooltip. */
      disabledReason?: string;
    }
  | { separator: true }
  | { heading: string };

export interface FileMenuOptions {
  path: string;
  /** Desktop only: the server viewer can't reveal or copy server-side paths. */
  desktop: boolean;
  onopeninlibrary?: () => void;
  onshowonatlas?: () => void;
}

/** "Open in Library", "Show on Sky Atlas", "Show in folder", "Copy path" — whichever apply. */
export function fileMenuItems(o: FileMenuOptions): ActionMenuItem[] {
  const items: ActionMenuItem[] = [];
  if (o.onopeninlibrary) items.push({ label: "Open in Library", run: o.onopeninlibrary });
  if (o.onshowonatlas) items.push({ label: "Show on Sky Atlas", run: o.onshowonatlas });
  if (o.desktop) {
    items.push(
      { label: "Show in folder", run: () => void revealPath(o.path) },
      { label: "Copy path", run: () => void copyPath(o.path) },
    );
  }
  return items;
}

/** Drops leading/trailing/doubled separators left by optional sections. */
export function tidy(items: ActionMenuItem[]): ActionMenuItem[] {
  const out: ActionMenuItem[] = [];
  for (const it of items) {
    const sep = "separator" in it;
    const prev = out[out.length - 1];
    if (sep && (!prev || "separator" in prev || "heading" in prev)) continue;
    out.push(it);
  }
  while (out.length && "separator" in out[out.length - 1]) out.pop();
  return out;
}
