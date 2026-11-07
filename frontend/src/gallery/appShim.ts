// ── `$app` for the gallery bundle ───────────────────────────────────────────
// The Sky Atlas and the Slideshow import their data calls from `$app`, which
// in the desktop app is the generated bindings of the whole App. The gallery
// build (vite.gallery.config.ts) points `$app` here instead: the same names,
// answered from the gallery service. Anything else the shared code imports
// but a visitor can never reach (file manager, preferences) is a stub.

import type * as app from "$models/app";
import { getCatalog, getGallery, getImageSizes } from "./api";
import { toAtlasEntry, toLibraryFrame } from "./adapt";
import type { GalleryFormat } from "./types";

/**
 * The format the visitor is looking at. The server sends both; the Sky Atlas
 * and the Slideshow only know how to show one set, so they are given that one.
 * GalleryApp sets this before (re)mounting them whenever the choice changes.
 */
let activeFormat: GalleryFormat = "fits";

export function setActiveFormat(f: GalleryFormat) {
  activeFormat = f;
}

export async function GetAtlasIndex(): Promise<app.AtlasIndexEntry[]> {
  return (await getGallery()).sky.filter((i) => i.format === activeFormat).map(toAtlasEntry);
}

export function GetAtlasFrameSizes(paths: string[]) {
  return getImageSizes(paths);
}

export function GetCatalog() {
  return getCatalog();
}

/** The slideshow asks for "processed" (FITS) and "image" (PNG/JPEG/TIFF) separately; only the active format has any. */
export async function GetLibraryFramesByType(
  _rootPath: string,
  frameType: string,
): Promise<app.LibraryFrame[]> {
  const { images } = await getGallery();
  const fits = frameType === "processed";
  if (!fits && frameType !== "image") return [];
  const wanted = fits ? "fits" : "raster";
  return images.filter((i) => i.format === wanted && i.format === activeFormat).map(toLibraryFrame);
}

// ── Not available to visitors ───────────────────────────────────────────────

const unavailable = () => Promise.reject(new Error("not available in the gallery"));

export const GetLibraryFrames = () => Promise.resolve([] as app.LibraryFrame[]);
export const RevealPath = unavailable;
export const OpenFolder = unavailable;
export const LoadPrefs = () => Promise.resolve({});
export const SetPref = () => Promise.resolve();
