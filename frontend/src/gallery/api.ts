// ── The gallery service ─────────────────────────────────────────────────────
// The server binds one Go service, internal/app.Gallery, and nothing else
// (see main_server.go). It has no generated bindings, so it is called by name.

import { Call } from "@wailsio/runtime";
import type * as app from "$models/app";
import type { GalleryInfo } from "./types";

const SERVICE = "github.com/hawawa4/eirin/internal/app.Gallery";

function call<T>(method: string, ...args: unknown[]): Promise<T> {
  return Call.ByName(`${SERVICE}.${method}`, ...args) as unknown as Promise<T>;
}

let inFlight: Promise<GalleryInfo> | null = null;

/**
 * The gallery: its texts and pictures. The Sky Atlas and the Slideshow each
 * ask for it when they load; asking while a request is out shares the reply.
 */
export function getGallery(): Promise<GalleryInfo> {
  if (!inFlight) {
    const p = call<GalleryInfo>("GetGallery").finally(() => {
      if (inFlight === p) inFlight = null;
    });
    inFlight = p;
  }
  return inFlight;
}

export function getCatalog(): Promise<app.CatalogObject[]> {
  return call("GetCatalog");
}

/** Pixel sizes by opaque id; pictures that can't be read are left out. */
export function getImageSizes(paths: string[]): Promise<Record<string, app.AtlasFrameSize>> {
  return call("GetImageSizes", paths);
}
