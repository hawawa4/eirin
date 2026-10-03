// ── Preview images for overlaid frames ──────────────────────────────────────
// Previews are rendered once into offscreen canvases and drawn at footprint
// size each frame. They are large (up to 2048 px RGBA), so the cache keeps the
// overlaid frames plus a few recently viewed ones and evicts the rest.

import type * as app from "$models/app";
import { GeneratePreviewRawSized, LoadRasterImage } from "$app";
import { renderStretched } from "../stretchPreview";

const PREVIEW_MAX_SIZE = 2048;
/** Extra (non-overlaid) previews kept around for quick re-selection. */
const SPARE_PREVIEWS = 4;

function isRasterFile(path: string): boolean {
  return path.toLowerCase().endsWith(".png");
}

function loadRaster(path: string): Promise<HTMLCanvasElement> {
  return LoadRasterImage(path).then(
    (dataUrl: string) =>
      new Promise<HTMLCanvasElement>((resolve, reject) => {
        const imgEl = new Image();
        imgEl.onload = () => {
          const c = document.createElement("canvas");
          c.width = imgEl.naturalWidth;
          c.height = imgEl.naturalHeight;
          c.getContext("2d")!.drawImage(imgEl, 0, 0);
          resolve(c);
        };
        imgEl.onerror = () => reject(new Error("could not decode image"));
        imgEl.src = dataUrl;
      }),
  );
}

async function loadFits(entry: app.AtlasIndexEntry): Promise<HTMLCanvasElement> {
  // Rows arrive top-down (the Go side flips FITS' bottom-up row order).
  const result = await GeneratePreviewRawSized(entry.nasPath, PREVIEW_MAX_SIZE);
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
    linear ? 0 : 2,
  );
}

export class PreviewCache {
  private readonly images = new Map<string, HTMLCanvasElement>();
  private readonly loading = new Set<string>();
  private readonly failed = new Set<string>();
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
    for (const e of entries) {
      this.touch(e.nasPath);
      if (this.images.has(e.nasPath) || this.loading.has(e.nasPath) || this.failed.has(e.nasPath))
        continue;
      this.load(e);
    }
    // Evict least-recently-used previews that aren't overlaid, beyond the spare budget.
    const spare = this.lru.filter((p) => !wanted.has(p) && this.images.has(p));
    for (const p of spare.slice(0, Math.max(0, spare.length - SPARE_PREVIEWS))) {
      this.images.delete(p);
      this.lru = this.lru.filter((x) => x !== p);
    }
  }

  clear(): void {
    this.images.clear();
    this.loading.clear();
    this.failed.clear();
    this.lru = [];
    this.onChange();
  }

  private touch(path: string): void {
    this.lru = this.lru.filter((p) => p !== path);
    this.lru.push(path);
  }

  private load(entry: app.AtlasIndexEntry): void {
    const path = entry.nasPath;
    this.loading.add(path);
    this.onChange();
    const p = isRasterFile(path) ? loadRaster(path) : loadFits(entry);
    p.then((c) => {
      if (this.loading.has(path)) this.images.set(path, c);
    })
      .catch((err: unknown) => {
        if (!this.loading.has(path)) return;
        this.failed.add(path);
        this.onError(entry, err);
      })
      .finally(() => {
        this.loading.delete(path);
        this.onChange();
      });
  }
}
