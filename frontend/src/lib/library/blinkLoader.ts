// ── Blink preview loader ────────────────────────────────────────────────────
// Keeps recently shown Blink frames (an LRU, so looping over a set doesn't
// re-read the NAS) and preloads around the current frame, nearest first, with
// a few requests in flight. Images are decoded before they count as ready, so
// switching frames never waits on the browser either.

export type LoadPreview = (path: string, level: number) => Promise<string>;

export class BlinkLoader {
  /** key → data URL ("" = failed). Map order is the LRU order (oldest first). */
  private readonly ready = new Map<string, string>();
  private readonly inflight = new Set<string>();
  private queue: { path: string; level: number }[] = [];

  constructor(
    private readonly load: LoadPreview,
    /** Called whenever a frame finishes loading. */
    private readonly onchange: () => void,
    private readonly capacity = 120,
    private readonly concurrency = 4,
  ) {}

  private static key(path: string, level: number): string {
    return `${level}\u0000${path}`;
  }

  /** The frame's data URL ("" if it failed), or undefined while not loaded. */
  get(path: string, level: number): string | undefined {
    const k = BlinkLoader.key(path, level);
    const url = this.ready.get(k);
    if (url !== undefined) {
      this.ready.delete(k); // refresh LRU position
      this.ready.set(k, url);
    }
    return url;
  }

  /** Loaded or failed: either way there's nothing left to wait for. */
  settled(path: string, level: number): boolean {
    return this.ready.has(BlinkLoader.key(path, level));
  }

  loading(path: string, level: number): boolean {
    return this.inflight.has(BlinkLoader.key(path, level));
  }

  /** Replaces the preload queue; `paths` should be nearest-first. */
  want(paths: string[], level: number): void {
    this.queue = paths.map((path) => ({ path, level }));
    this.pump();
  }

  private pump(): void {
    while (this.inflight.size < this.concurrency && this.queue.length > 0) {
      const { path, level } = this.queue.shift()!;
      const k = BlinkLoader.key(path, level);
      if (this.ready.has(k) || this.inflight.has(k)) continue;
      this.inflight.add(k);
      this.fetch(path, level).then((url) => {
        this.inflight.delete(k);
        this.ready.set(k, url);
        while (this.ready.size > this.capacity) {
          this.ready.delete(this.ready.keys().next().value!);
        }
        this.onchange();
        this.pump();
      });
    }
  }

  private async fetch(path: string, level: number): Promise<string> {
    try {
      const url = await this.load(path, level);
      const img = new Image();
      img.src = url;
      await img.decode().catch(() => {});
      return url;
    } catch {
      return "";
    }
  }
}

/** Indices to preload around `i`: the frame itself, then mostly forward. */
export function preloadOrder(i: number, n: number, ahead: number, behind: number): number[] {
  const out = [i];
  for (let k = 1; k <= Math.max(ahead, behind); k++) {
    if (k <= ahead) out.push((i + k) % n);
    if (k <= behind) out.push((((i - k) % n) + n) % n);
  }
  return [...new Set(out)];
}
