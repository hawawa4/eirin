// ── Preview images for overlaid frames ──────────────────────────────────────
// Previews are rendered once into offscreen canvases and drawn at footprint
// size each frame. They are large (up to 2048 px RGBA), so the cache keeps the
// overlaid frames plus a few recently viewed ones and evicts the rest.

import type * as app from "$models/app";
import { GeneratePreviewRawSized, LoadRasterImage } from "$app";
import { renderStretched } from "../stretchPreview";
import { isRasterFile } from "../utils";

/** Extra (non-overlaid) previews kept around for quick re-selection. */
const SPARE_PREVIEWS = 4;
/** Previews fetched/decoded at the same time (a bulk overlay would otherwise flood the backend). */
const MAX_CONCURRENT = 3;

/**
 * Longest preview side for an overlay of `count` frames. Each preview is an
 * RGBA canvas (16 MB at 2048 px), so a bulk overlay trades resolution for
 * memory; shrinking the overlay again reloads the survivors at full size.
 */
export function previewSizeFor(count: number): number {
  if (count <= 6) return 2048;
  if (count <= 24) return 1024;
  return 512;
}

function loadRaster(path: string, maxSize: number): Promise<HTMLCanvasElement> {
  return LoadRasterImage(path).then(
    (dataUrl: string) =>
      new Promise<HTMLCanvasElement>((resolve, reject) => {
        const imgEl = new Image();
        imgEl.onload = () => {
          const k = Math.min(1, maxSize / Math.max(imgEl.naturalWidth, imgEl.naturalHeight));
          const c = document.createElement("canvas");
          c.width = Math.max(1, Math.round(imgEl.naturalWidth * k));
          c.height = Math.max(1, Math.round(imgEl.naturalHeight * k));
          c.getContext("2d")!.drawImage(imgEl, 0, 0, c.width, c.height);
          resolve(c);
        };
        imgEl.onerror = () => reject(new Error("could not decode image"));
        imgEl.src = dataUrl;
      }),
  );
}

async function loadFits(entry: app.AtlasIndexEntry, maxSize: number): Promise<HTMLCanvasElement> {
  // Rows arrive top-down (the Go side flips FITS' bottom-up row order).
  const result = await GeneratePreviewRawSized(entry.nasPath, maxSize);
  const bin = atob(result.data);
  const u8 = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) u8[i] = bin.charCodeAt(i);
  const f32 = new Float32Array(u8.buffer);
  const linear = entry.frameType === "processed" || entry.frameType === "image";
  return renderStretched(
    f32,
    result.width,
    result.height,
    result.channels,
    result.stats,
    result.balance,
    linear ? 0 : 2,
  );
}

export class PreviewCache {
  private readonly images = new Map<string, HTMLCanvasElement>();
  private readonly loading = new Set<string>();
  private readonly failed = new Set<string>();
  /** Longest side each cached image was loaded at. */
  private readonly loadedSize = new Map<string, number>();
  /** Loads waiting for a free slot, in priority order. */
  private queue: { entry: app.AtlasIndexEntry; size: number }[] = [];
  private running = 0;
  /** Most-recently-used order (last = newest). */
  private lru: string[] = [];

  constructor(
    private readonly onChange: () => void,
    private readonly onError: (entry: app.AtlasIndexEntry, err: unknown) => void,
  ) {}

  get(path: string): HTMLCanvasElement | undefined {
    return this.images.get(path);
  }

  isLoading(path: string): boolean {
    return this.loading.has(path);
  }

  get loadingCount(): number {
    return this.loading.size;
  }

  /** Allows a previously failed preview to be retried (e.g. the user clicked it again). */
  retry(path: string): void {
    this.failed.delete(path);
  }

  /** Ensures previews for `entries` are loaded/loading, and trims the cache. */
  sync(entries: app.AtlasIndexEntry[]): void {
    const wanted = new Set(entries.map((e) => e.nasPath));
    const size = previewSizeFor(entries.length);
    // Drop queued loads nobody wants any more (the overlay changed under them).
    const dropped = this.queue.filter((q) => !wanted.has(q.entry.nasPath));
    if (dropped.length > 0) {
      this.queue = this.queue.filter((q) => wanted.has(q.entry.nasPath));
      for (const q of dropped) this.loading.delete(q.entry.nasPath);
      this.onChange();
    }
    for (const e of entries) {
      const path = e.nasPath;
      this.touch(path);
      if (this.loading.has(path) || this.failed.has(path)) continue;
      const have = this.loadedSize.get(path);
      if (this.images.has(path) && (have === undefined || have >= size)) continue;
      this.enqueue(e, size);
    }
    // Evict least-recently-used previews that aren't overlaid, beyond the spare budget.
    const spare = this.lru.filter((p) => !wanted.has(p) && this.images.has(p));
    for (const p of spare.slice(0, Math.max(0, spare.length - SPARE_PREVIEWS))) {
      this.images.delete(p);
      this.loadedSize.delete(p);
      this.lru = this.lru.filter((x) => x !== p);
    }
  }

  clear(): void {
    this.images.clear();
    this.loading.clear();
    this.failed.clear();
    this.loadedSize.clear();
    this.queue = [];
    this.lru = [];
    this.onChange();
  }

  private touch(path: string): void {
    this.lru = this.lru.filter((p) => p !== path);
    this.lru.push(path);
  }

  private enqueue(entry: app.AtlasIndexEntry, size: number): void {
    this.loading.add(entry.nasPath);
    this.queue.push({ entry, size });
    this.onChange();
    this.pump();
  }

  private pump(): void {
    while (this.running < MAX_CONCURRENT && this.queue.length > 0) {
      const { entry, size } = this.queue.shift()!;
      this.run(entry, size);
    }
  }

  private run(entry: app.AtlasIndexEntry, size: number): void {
    const path = entry.nasPath;
    this.running++;
    const p = isRasterFile(path) ? loadRaster(path, size) : loadFits(entry, size);
    p.then((c) => {
      if (!this.loading.has(path)) return;
      this.images.set(path, c);
      this.loadedSize.set(path, size);
    })
      .catch((err: unknown) => {
        if (!this.loading.has(path)) return;
        this.failed.add(path);
        this.onError(entry, err);
      })
      .finally(() => {
        this.running--;
        this.loading.delete(path);
        this.onChange();
        this.pump();
      });
  }
}
