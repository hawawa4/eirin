// ── Routing by URL hash ─────────────────────────────────────────────────────
// A gallery is meant to be linked to: "#/atlas", "#/slideshow", and
// "#/image/<id>" for one picture opened on the grid. No hash is the
// configured home view.

import type { GalleryView } from "./types";

export interface Route {
  view: GalleryView | null;
  /** Opaque id ("/g/….jpg") of the picture open in the lightbox. */
  image: string | null;
}

export function parseHash(hash: string): Route {
  const parts = hash.replace(/^#\/?/, "").split("/").filter(Boolean);
  if (parts[0] === "image" && /^[\w.-]+$/.test(parts[1] ?? "")) {
    return { view: "grid", image: `/g/${parts[1]}` };
  }
  if (parts[0] === "grid" || parts[0] === "atlas" || parts[0] === "slideshow") {
    return { view: parts[0], image: null };
  }
  return { view: null, image: null };
}

/** The hash for a view, or for a picture on the grid. */
export function hashFor(view: GalleryView, image?: string): string {
  if (image) return `#/image/${image.slice(image.lastIndexOf("/") + 1)}`;
  return `#/${view}`;
}
