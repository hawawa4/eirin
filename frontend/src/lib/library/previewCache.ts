// ── Preview cache ───────────────────────────────────────────────────────────
// Small LRU of decoded FITS previews (pixels + stats + header) keyed by path, so
// stepping back and forth through frames — and the row PreviewPane prefetches —
// render instantly. Each entry holds a decoded RGBA float buffer (tens of MB),
// so the cache is deliberately tiny.

import type * as fits from "$models/fits";
import { GeneratePreviewRawSized, ReadFITSHeader } from "$app";

export const PREVIEW_MAX_SIZE = 2048;
const CAPACITY = 3;

export interface HistBins {
  r: Float32Array;
  g: Float32Array;
  b: Float32Array;
  channels: number;
}

export interface DecodedPreview {
  path: string;
  header: fits.FITSHeader | null;
  pixels: Float32Array;
  width: number;
  height: number;
  channels: number;
  stats: fits.ChannelStats[];
  /** Background-balance gains per channel (see lib/stretchPreview.ts). */
  balance: number[];
  /** Filled lazily by the consumer the first time the histogram is needed. */
  hist: HistBins | null;
}

const cache = new Map<string, DecodedPreview>();
const inflight = new Map<string, Promise<DecodedPreview>>();

function touch(entry: DecodedPreview) {
  cache.delete(entry.path);
  cache.set(entry.path, entry);
  while (cache.size > CAPACITY) {
    const oldest = cache.keys().next().value;
    if (oldest === undefined) break;
    cache.delete(oldest);
  }
}

function decode(b64: string): Float32Array {
  const bin = atob(b64);
  const u8 = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) u8[i] = bin.charCodeAt(i);
  return new Float32Array(u8.buffer);
}

/** Returns the cached preview for `path` without fetching (and marks it recently used). */
export function peekPreview(path: string): DecodedPreview | null {
  const hit = cache.get(path);
  if (hit) touch(hit);
  return hit ?? null;
}

/**
 * Loads (or reuses) the decoded preview for `path`. Concurrent calls share one request.
 * Rejects if the pixel data can't be generated; a missing header is tolerated.
 */
export function loadPreview(path: string): Promise<DecodedPreview> {
  const hit = peekPreview(path);
  if (hit) return Promise.resolve(hit);
  const pending = inflight.get(path);
  if (pending) return pending;

  const p = Promise.allSettled([
    ReadFITSHeader(path),
    GeneratePreviewRawSized(path, PREVIEW_MAX_SIZE),
  ]).then(([hdr, raw]) => {
    inflight.delete(path);
    if (raw.status === "rejected") {
      throw raw.reason instanceof Error ? raw.reason : new Error(String(raw.reason));
    }
    const r = raw.value;
    const entry: DecodedPreview = {
      path,
      header: hdr.status === "fulfilled" ? hdr.value : null,
      pixels: decode(r.data),
      width: r.width,
      height: r.height,
      channels: r.channels,
      stats: r.stats ?? [],
      balance: r.balance ?? [],
      hist: null,
    };
    touch(entry);
    return entry;
  });
  inflight.set(path, p);
  return p;
}

/** Warms the cache for `path` in the background; errors are ignored. */
export function prefetchPreview(path: string): void {
  if (cache.has(path) || inflight.has(path)) return;
  loadPreview(path).catch(() => {
    /* prefetch is best-effort */
  });
}

/** Drops a path (e.g. after the file was deleted or renamed). */
export function forgetPreview(path: string): void {
  cache.delete(path);
}
