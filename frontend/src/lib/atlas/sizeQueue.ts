// ── Lazy frame-size fetching ────────────────────────────────────────────────
// Frame pixel sizes need a FITS header read per file (on a NAS), so they are
// fetched only for frames in view, with bounded concurrency. Each `request()`
// REPLACES the pending queue — after a pan/zoom, frames that scrolled away are
// dropped instead of being fetched anyway.

import type { FrameSize } from "./footprint";

export class SizeQueue {
  private readonly sizes = new Map<string, FrameSize>();
  private readonly failed = new Set<string>();
  private readonly inflight = new Set<string>();
  private pending: string[] = [];
  private generation = 0;

  constructor(
    private readonly fetchSize: (path: string) => Promise<{ width: number; height: number }>,
    private readonly onChange: () => void,
    private readonly concurrency = 4,
  ) {}

  get(path: string): FrameSize | undefined {
    return this.sizes.get(path);
  }

  /** Number of fetches queued or running (for a loading indicator). */
  get busy(): number {
    return this.pending.length + this.inflight.size;
  }

  /** Replaces the queue with `paths` (in priority order), skipping known/failed/in-flight ones. */
  request(paths: string[]): void {
    this.pending = [...new Set(paths)].filter(
      (p) => !this.sizes.has(p) && !this.failed.has(p) && !this.inflight.has(p),
    );
    this.pump();
    this.onChange();
  }

  /** Forgets everything (e.g. the library root changed). In-flight results are discarded. */
  reset(): void {
    this.generation++;
    this.sizes.clear();
    this.failed.clear();
    this.inflight.clear();
    this.pending = [];
    this.onChange();
  }

  private pump(): void {
    while (this.inflight.size < this.concurrency && this.pending.length > 0) {
      const path = this.pending.shift()!;
      const gen = this.generation;
      this.inflight.add(path);
      this.fetchSize(path)
        .then((sz) => {
          if (gen !== this.generation) return;
          if (sz.width > 0 && sz.height > 0) {
            this.sizes.set(path, { width: sz.width, height: sz.height });
          } else {
            this.failed.add(path);
          }
        })
        .catch(() => {
          if (gen === this.generation) this.failed.add(path);
        })
        .finally(() => {
          if (gen !== this.generation) return;
          this.inflight.delete(path);
          this.onChange();
          this.pump();
        });
    }
  }
}
