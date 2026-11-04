// ── Lazy frame-size fetching ────────────────────────────────────────────────
// Frame pixel sizes need a FITS header read per file (on a NAS), so they are
// fetched only for frames in view, in batches (one round trip each, read in
// parallel by the backend) with bounded concurrency. Each `request()`
// REPLACES the pending queue — after a pan/zoom, frames that scrolled away are
// dropped instead of being fetched anyway.

import type { FrameSize } from "./footprint";

/** Paths per backend call: big enough to cut round trips, small enough to show progress. */
const BATCH = 48;

type SizeMap = { [path: string]: { width: number; height: number } | undefined };

export class SizeQueue {
  private readonly sizes = new Map<string, FrameSize>();
  private readonly failed = new Set<string>();
  private readonly inflight = new Set<string>();
  private pending: string[] = [];
  private batches = 0;
  private generation = 0;

  constructor(
    private readonly fetchSizes: (paths: string[]) => Promise<SizeMap>,
    private readonly onChange: () => void,
    private readonly concurrency = 2,
  ) {}

  get(path: string): FrameSize | undefined {
    return this.sizes.get(path);
  }

  /** The size couldn't be read, so waiting for it is pointless. */
  isFailed(path: string): boolean {
    return this.failed.has(path);
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
    this.batches = 0;
    this.onChange();
  }

  private pump(): void {
    while (this.batches < this.concurrency && this.pending.length > 0) {
      const batch = this.pending.splice(0, BATCH);
      const gen = this.generation;
      this.batches++;
      for (const p of batch) this.inflight.add(p);
      this.fetchSizes(batch)
        .then((got) => {
          if (gen !== this.generation) return;
          for (const p of batch) {
            const sz = got?.[p];
            if (sz && sz.width > 0 && sz.height > 0) {
              this.sizes.set(p, { width: sz.width, height: sz.height });
            } else {
              this.failed.add(p);
            }
          }
        })
        .catch(() => {
          if (gen === this.generation) for (const p of batch) this.failed.add(p);
        })
        .finally(() => {
          if (gen !== this.generation) return;
          this.batches--;
          for (const p of batch) this.inflight.delete(p);
          this.onChange();
          this.pump();
        });
    }
  }
}
