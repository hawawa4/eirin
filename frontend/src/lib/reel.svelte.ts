// ── Reel: a list that steps to its next item every few seconds ──────────────
// The timing shared by the slideshow and the Sky Atlas tour. The reel only
// holds the position and the timer; the views decide how an item is shown.

const PREF_INTERVAL = "eirin.reelIntervalMs";

/** Offered dwell times, in seconds. */
export const REEL_INTERVALS_S = [3, 5, 8, 12, 20, 30];
const DEFAULT_INTERVAL_MS = 8000;

function loadInterval(): number {
  try {
    const n = Number(localStorage.getItem(PREF_INTERVAL));
    if (REEL_INTERVALS_S.includes(n / 1000)) return n;
  } catch {
    /* storage unavailable */
  }
  return DEFAULT_INTERVAL_MS;
}

export class Reel<T> {
  index = $state(0);
  /** The user's play/pause choice. The timer also needs `run(true)`. */
  playing = $state(true);
  intervalMs = $state(loadInterval());

  private timer: ReturnType<typeof setTimeout> | null = null;
  private running = false;

  constructor(private readonly items: () => readonly T[]) {}

  get length(): number {
    return this.items().length;
  }

  /** Index of the current item, kept in range when the list shrinks. */
  get position(): number {
    const n = this.length;
    return n === 0 ? 0 : Math.min(this.index, n - 1);
  }

  get current(): T | undefined {
    return this.items()[this.position];
  }

  /** The item after the current one (wrapping), if it is a different item. */
  get upcoming(): T | undefined {
    const n = this.length;
    return n > 1 ? this.items()[(this.position + 1) % n] : undefined;
  }

  next(): void {
    this.goTo(this.position + 1);
  }

  prev(): void {
    this.goTo(this.position - 1);
  }

  goTo(i: number): void {
    const n = this.length;
    this.index = n === 0 ? 0 : ((i % n) + n) % n;
    this.arm();
  }

  toggle(): void {
    this.playing = !this.playing;
    this.arm();
  }

  setIntervalMs(ms: number): void {
    this.intervalMs = ms;
    try {
      localStorage.setItem(PREF_INTERVAL, String(ms));
    } catch {
      /* storage unavailable */
    }
    this.arm();
  }

  /** Starts or stops the timer (e.g. while the view is hidden). */
  run(on: boolean): void {
    this.running = on;
    this.arm();
  }

  /** (Re)starts the countdown to the next item. */
  private arm(): void {
    if (this.timer !== null) clearTimeout(this.timer);
    this.timer = null;
    if (!this.running || !this.playing) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      this.next();
    }, this.intervalMs);
  }
}
