// ── Preview images for overlaid frames ──────────────────────────────────────
// Each overlaid frame asks for the preview level that matches its size on
// screen (see targetLevels in SkyAtlas): small when zoomed out, sharper —
// up to the native image — as the user zooms in. A frame with nothing to
// show yet gets a small level first so it appears quickly, then the sharper
// one replaces it. Decoded bitmaps are large, so the cache keeps the overlaid
// frames plus a few recently viewed ones, and steps frames that no longer
// need their resolution back down once it holds too many pixels.

import type * as app from "$models/app";
import { NATIVE, fetchBitmap, levelRank, previewUrl, stretchFor } from "../images";

/** Extra (non-overlaid) previews kept around for quick re-selection. */
const SPARE_PREVIEWS = 4;
/** Previews fetched at the same time (a bulk overlay would otherwise flood the backend). */
const MAX_CONCURRENT = 4;
/** Level a frame with nothing on screen yet starts at. */
const FIRST_LEVEL = 512;
/**
 * Decoded pixels held before frames shown smaller than they're loaded are
 * stepped down, and only the frame nearest the centre loads past TIGHT_LEVEL.
 */
const PIXEL_BUDGET = 120e6;
const TIGHT_LEVEL = 2048;

/** A frame's wanted preview level, in priority order. */
export interface PreviewTarget {
  entry: app.AtlasIndexEntry;
  level: number;
}

interface Loaded {
  bmp: ImageBitmap;
  level: number;
}

interface Load {
  entry: app.AtlasIndexEntry;
  level: number;
  /** A replacement for an image already shown (not a first load). */
  upgrade: boolean;
  ctrl: AbortController;
}

export class PreviewCache {
  private readonly images = new Map<string, Loaded>();
  /** Queued or running loads, one per path. */
  private readonly loads = new Map<string, Load>();
  private queue: Load[] = [];
  private running = 0;
  private readonly failed = new Set<string>();
  /** Targets of the last sync, by path. */
  private wanted = new Map<string, PreviewTarget>();
  /** Most-recently-used order (last = newest). */
  private lru: string[] = [];

  constructor(
    private readonly onChange: () => void,
    private readonly onError: (entry: app.AtlasIndexEntry, err: unknown) => void,
  ) {}

  get(path: string): ImageBitmap | undefined {
    return this.images.get(path)?.bmp;
  }

  /** Nothing is shown for `path` yet, but it's on its way. */
  isLoading(path: string): boolean {
    return this.loads.has(path) && !this.images.has(path);
  }

  /** Frames waiting for their first image (sharper replacements aren't counted). */
  get loadingCount(): number {
    let n = 0;
    for (const p of this.loads.keys()) if (!this.images.has(p)) n++;
    return n;
  }

  /** Allows a previously failed preview to be retried (e.g. the user clicked it again). */
  retry(path: string): void {
    this.failed.delete(path);
  }

  /** Loads what `targets` need, drops what nobody wants, and trims the cache. */
  sync(targets: PreviewTarget[]): void {
    // Short of memory, only the most important frame gets past TIGHT_LEVEL.
    if (this.overBudget()) {
      targets = targets.map((t, i) =>
        i > 0 && levelRank(t.level) > TIGHT_LEVEL ? { ...t, level: TIGHT_LEVEL } : t,
      );
    }
    this.wanted = new Map(targets.map((t) => [t.entry.nasPath, t]));
    let changed = false;

    // Loads nobody wants any more (the overlay changed under them).
    for (const [path, load] of this.loads) {
      if (!this.wanted.has(path)) {
        this.cancel(path, load);
        changed = true;
      }
    }

    for (const t of targets) {
      this.touch(t.entry.nasPath);
      if (this.request(t)) changed = true;
    }
    if (this.trim()) changed = true;
    // Most important first: first loads, then in target order.
    const order = new Map(targets.map((t, i) => [t.entry.nasPath, i]));
    this.queue.sort(
      (a, b) =>
        Number(a.upgrade) - Number(b.upgrade) ||
        (order.get(a.entry.nasPath) ?? 0) - (order.get(b.entry.nasPath) ?? 0),
    );
    this.pump();
    if (changed) this.onChange();
  }

  clear(): void {
    for (const [path, load] of this.loads) this.cancel(path, load);
    for (const { bmp } of this.images.values()) bmp.close();
    this.images.clear();
    this.failed.clear();
    this.wanted.clear();
    this.queue = [];
    this.lru = [];
    this.onChange();
  }

  /** Queues the load `t` needs next, if any. Returns whether anything changed. */
  private request(t: PreviewTarget): boolean {
    const path = t.entry.nasPath;
    if (this.failed.has(path)) return false;
    const have = this.images.get(path);
    const level =
      have || levelRank(t.level) <= FIRST_LEVEL
        ? t.level
        : Math.min(FIRST_LEVEL, levelRank(t.level));
    if (have && have.level === level) return false;
    // Already sharp enough, unless it's a step down to save memory (see trim).
    if (have && levelRank(have.level) > levelRank(level) && !this.overBudget()) return false;
    const pending = this.loads.get(path);
    if (pending) {
      if (pending.level === level) return false;
      this.cancel(path, pending); // a sharper (or smaller) one is wanted now
    }
    const load: Load = { entry: t.entry, level, upgrade: !!have, ctrl: new AbortController() };
    this.loads.set(path, load);
    this.queue.push(load);
    return true;
  }

  private cancel(path: string, load: Load): void {
    load.ctrl.abort();
    this.loads.delete(path);
    this.queue = this.queue.filter((q) => q !== load);
  }

  private overBudget(): boolean {
    let px = 0;
    for (const { bmp } of this.images.values()) px += bmp.width * bmp.height;
    return px > PIXEL_BUDGET;
  }

  /** Evicts spare previews beyond the budget. Returns whether anything was dropped. */
  private trim(): boolean {
    const spare = this.lru.filter((p) => !this.wanted.has(p) && this.images.has(p));
    const drop = spare.slice(0, Math.max(0, spare.length - SPARE_PREVIEWS));
    for (const p of drop) {
      this.images.get(p)?.bmp.close();
      this.images.delete(p);
    }
    if (drop.length > 0) {
      const gone = new Set(drop);
      this.lru = this.lru.filter((p) => !gone.has(p));
    }
    return drop.length > 0;
  }

  private touch(path: string): void {
    const i = this.lru.indexOf(path);
    if (i >= 0) this.lru.splice(i, 1);
    this.lru.push(path);
  }

  private pump(): void {
    while (this.running < MAX_CONCURRENT && this.queue.length > 0) {
      this.run(this.queue.shift()!);
    }
  }

  private run(load: Load): void {
    const { entry, level, ctrl } = load;
    const path = entry.nasPath;
    this.running++;
    fetchBitmap(previewUrl(path, level, stretchFor(entry.frameType)), ctrl.signal)
      .then((bmp) => {
        if (this.loads.get(path) !== load) {
          bmp.close(); // cancelled or superseded meanwhile
          return;
        }
        this.images.get(path)?.bmp.close();
        // Smaller than the level asked for: that's the whole image, nothing sharper exists.
        const whole = Math.max(bmp.width, bmp.height) < levelRank(level);
        this.images.set(path, { bmp, level: whole ? NATIVE : level });
      })
      .catch((err: unknown) => {
        if (ctrl.signal.aborted || this.loads.get(path) !== load) return;
        // A failed sharper version leaves the shown one; a failed first load is reported.
        if (!this.images.has(path)) {
          this.failed.add(path);
          this.onError(entry, err);
        }
      })
      .finally(() => {
        this.running--;
        if (this.loads.get(path) === load) this.loads.delete(path);
        // The frame may want a sharper level now that something is shown.
        const t = this.wanted.get(path);
        if (t && !ctrl.signal.aborted) this.request(t);
        this.pump();
        this.onChange();
      });
  }
}
