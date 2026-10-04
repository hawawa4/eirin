// ── Server-rendered previews (read-only viewer) ─────────────────────────────
// The server viewer reaches the backend over the network, so previews come as
// ~1 MB JPEGs rendered on the server (GetViewerPreview) rather than the raw
// float pixels the desktop stretches locally. Kept in a small LRU so stepping
// back and forth doesn't refetch.

import { GetViewerPreview } from "$app";
import type * as fits from "$models/fits";

const MAX_ENTRIES = 12;

const cache = new Map<string, fits.RenderedPreview>();
const inflight = new Map<string, Promise<fits.RenderedPreview>>();

const keyOf = (path: string, level: number) => `${path}\u0000${level}`;

/** The preview for `path` at stretch `level` (ignored for raster images). */
export function loadViewerPreview(path: string, level: number): Promise<fits.RenderedPreview> {
  const key = keyOf(path, level);
  const hit = cache.get(key);
  if (hit) {
    cache.delete(key); // re-insert as most recently used
    cache.set(key, hit);
    return Promise.resolve(hit);
  }
  const pending = inflight.get(key);
  if (pending) return pending;
  const p = Promise.resolve(GetViewerPreview(path, level))
    .then((r) => {
      cache.set(key, r);
      while (cache.size > MAX_ENTRIES) cache.delete(cache.keys().next().value!);
      return r;
    })
    .finally(() => inflight.delete(key));
  inflight.set(key, p);
  return p;
}

/** Warms the cache in the background; errors are ignored. */
export function prefetchViewerPreview(path: string, level: number): void {
  loadViewerPreview(path, level).catch(() => {
    /* best-effort */
  });
}
