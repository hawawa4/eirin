// ── Preview cache ───────────────────────────────────────────────────────────
// Small LRU of decoded FITS previews (pixels + stats + header) keyed by path, so
// stepping back and forth through frames — and the row PreviewPane prefetches —
// render instantly. Each entry holds a decoded RGBA float buffer (tens of MB),
// so the cache is deliberately tiny. Pixels come from the backend's binary raw
// route (internal/app/images.go), not an RPC: a fraction of the size, and
// abortable, so skipping quickly through rows doesn't queue a render for each.

import type * as fits from "$models/fits";
import type { ChannelStats } from "../stretchPreview";

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
  stats: ChannelStats[];
  /** Background-balance gains per channel (see lib/stretchPreview.ts). */
  balance: number[];
  /** Filled lazily by the consumer the first time the histogram is needed. */
  hist: HistBins | null;
}

const cache = new Map<string, DecodedPreview>();
const inflight = new Map<string, { promise: Promise<DecodedPreview>; ctrl: AbortController }>();
/** The frame on screen and the one warmed for next: other loads are abandoned. */
let shownPath: string | null = null;
let prefetchPath: string | null = null;

function touch(entry: DecodedPreview) {
  cache.delete(entry.path);
  cache.set(entry.path, entry);
  while (cache.size > CAPACITY) {
    const oldest = cache.keys().next().value;
    if (oldest === undefined) break;
    cache.delete(oldest);
  }
}

/** Cancels the load of `path` unless it's still wanted (shown or prefetched). */
function abandon(path: string | null) {
  if (!path || path === shownPath || path === prefetchPath) return;
  inflight.get(path)?.ctrl.abort();
  inflight.delete(path); // coming back to it starts a fresh load
}

interface RawMeta {
  width: number;
  height: number;
  channels: number;
  stats: ChannelStats[] | null;
  balance: number[] | null;
  header: fits.FITSHeader | null;
}

/**
 * Decodes the raw route's reply (fits.EncodeRawPreview): a JSON header, then
 * uint16 samples, expanded to the RGBA float32 the WebGL stretch and the
 * histogram use.
 */
function decodeRaw(path: string, buf: ArrayBuffer): DecodedPreview {
  const n = new DataView(buf).getUint32(0, true);
  const meta = JSON.parse(new TextDecoder().decode(new Uint8Array(buf, 4, n))) as RawMeta;
  const { width, height, channels } = meta;
  const samples = new Uint16Array(buf, 4 + n, width * height * channels);
  const pixels = new Float32Array(width * height * 4);
  const k = 1 / 65535;
  if (channels === 1) {
    for (let i = 0, o = 0; i < samples.length; i++, o += 4) {
      const v = samples[i] * k;
      pixels[o] = v;
      pixels[o + 1] = v;
      pixels[o + 2] = v;
      pixels[o + 3] = 1;
    }
  } else {
    for (let i = 0, o = 0; i < samples.length; i += 3, o += 4) {
      pixels[o] = samples[i] * k;
      pixels[o + 1] = samples[i + 1] * k;
      pixels[o + 2] = samples[i + 2] * k;
      pixels[o + 3] = 1;
    }
  }
  return {
    path,
    header: meta.header,
    pixels,
    width,
    height,
    channels,
    stats: meta.stats ?? [],
    balance: meta.balance ?? [],
    hist: null,
  };
}

function fetchPreview(path: string): Promise<DecodedPreview> {
  const pending = inflight.get(path);
  if (pending) return pending.promise;
  const ctrl = new AbortController();
  const url = `/eirin/raw?${new URLSearchParams({ path, size: String(PREVIEW_MAX_SIZE) })}`;
  const promise = fetch(url, { signal: ctrl.signal })
    .then(async (res) => {
      if (!res.ok) {
        const msg = (await res.text().catch(() => "")).trim();
        throw new Error(msg || `HTTP ${res.status}`);
      }
      const entry = decodeRaw(path, await res.arrayBuffer());
      touch(entry);
      return entry;
    })
    .finally(() => {
      if (inflight.get(path)?.ctrl === ctrl) inflight.delete(path);
    });
  inflight.set(path, { promise, ctrl });
  return promise;
}

/** Returns the cached preview for `path` without fetching (and marks it recently used). */
export function peekPreview(path: string): DecodedPreview | null {
  const hit = cache.get(path);
  if (hit) touch(hit);
  return hit ?? null;
}

/**
 * Loads (or reuses) the decoded preview for `path`, the frame now on screen:
 * a load for one the user already moved past is abandoned. Concurrent calls
 * share one request.
 */
export function loadPreview(path: string): Promise<DecodedPreview> {
  const before = shownPath;
  shownPath = path;
  abandon(before);
  const hit = peekPreview(path);
  if (hit) return Promise.resolve(hit);
  return fetchPreview(path);
}

/** Warms the cache for `path` (the likely next frame) in the background; errors are ignored. */
export function prefetchPreview(path: string): void {
  const before = prefetchPath;
  prefetchPath = path;
  abandon(before);
  if (cache.has(path)) return;
  fetchPreview(path).catch(() => {
    /* prefetch is best-effort */
  });
}

/** Drops a path (e.g. after the file was deleted or renamed). */
export function forgetPreview(path: string): void {
  cache.delete(path);
}
