// ── Gallery pictures in the shapes the app's views use ──────────────────────
// The Sky Atlas and the Slideshow are the desktop app's own components. They
// read `AtlasIndexEntry` / `LibraryFrame`; the gallery fills those in from its
// pictures, with the opaque id as the path, and a file name that is just that
// id (so they still tell FITS from PNG by its extension, and nothing real
// leaks).

import type * as app from "$models/app";
import type { GalleryImage } from "./types";

/** "/g/3f9a….jpg" → "3f9a….jpg". */
export function idName(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1);
}

export function toAtlasEntry(img: GalleryImage): app.AtlasIndexEntry {
  return {
    nasPath: img.path,
    name: idName(img.path),
    object: img.title,
    frameType: "processed",
    ra: img.ra,
    dec: img.dec,
    pixelScale: img.pixelScale,
    rotation: img.rotation,
    telescope: img.telescope,
  } as app.AtlasIndexEntry;
}

export function toLibraryFrame(img: GalleryImage): app.LibraryFrame {
  return {
    nasPath: img.path,
    fileName: idName(img.path),
    frameType: img.format === "fits" ? "processed" : "image",
    object: img.title,
    filter: img.filter,
    expTime: img.exposure,
    dateObs: img.date,
    telescope: img.telescope,
    isRejected: false,
    ra: img.ra,
    dec: img.dec,
    pixelScale: img.pixelScale,
    rotation: img.rotation,
    wcsSolved: img.solved,
  } as unknown as app.LibraryFrame;
}
