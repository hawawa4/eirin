// ── Drag inertia ────────────────────────────────────────────────────────────
// After a fast drag the map keeps gliding and slows down exponentially, like
// Google Maps / OpenLayers' DragPan kinetic.

/** Only the last stretch of the drag decides the release velocity. */
const SAMPLE_WINDOW_MS = 80;
/** Holding still this long before releasing means "stop here", not "fling". */
const MAX_IDLE_MS = 50;
/** Release speeds below this (px/ms) don't coast. */
const MIN_SPEED = 0.25;
/** Speed is capped so a jittery sample can't throw the view across the sky. */
const MAX_SPEED = 6;
/** Decay time constant: speed falls to 1/e after this many ms. */
export const DECAY_MS = 325;
/** Coasting stops once the speed drops below this (px/ms). */
export const STOP_SPEED = 0.02;

interface Sample {
  t: number;
  x: number;
  y: number;
}

export class DragTracker {
  private samples: Sample[] = [];

  reset(t: number, x: number, y: number) {
    this.samples = [{ t, x, y }];
  }

  add(t: number, x: number, y: number) {
    this.samples.push({ t, x, y });
    const cutoff = t - SAMPLE_WINDOW_MS;
    while (this.samples.length > 2 && this.samples[0].t < cutoff) this.samples.shift();
  }

  /** Release velocity in px/ms, or null when the drag shouldn't coast. */
  velocity(now: number): [number, number] | null {
    const s = this.samples;
    if (s.length < 2) return null;
    const last = s[s.length - 1];
    if (now - last.t > MAX_IDLE_MS) return null;
    const first = s.find((p) => p.t >= last.t - SAMPLE_WINDOW_MS) ?? s[0];
    const dt = last.t - first.t;
    if (dt <= 0) return null;
    let vx = (last.x - first.x) / dt;
    let vy = (last.y - first.y) / dt;
    const speed = Math.hypot(vx, vy);
    if (speed < MIN_SPEED) return null;
    if (speed > MAX_SPEED) {
      vx *= MAX_SPEED / speed;
      vy *= MAX_SPEED / speed;
    }
    return [vx, vy];
  }
}

/**
 * Advances a coast by `dt` ms. Returns the pixel offset to pan by and the new
 * velocity (the exact integral of an exponential decay, so frame rate doesn't
 * change how far it travels).
 */
export function coastStep(
  v: [number, number],
  dt: number,
): { dx: number; dy: number; v: [number, number] } {
  const k = Math.exp(-dt / DECAY_MS);
  const travel = DECAY_MS * (1 - k);
  return { dx: v[0] * travel, dy: v[1] * travel, v: [v[0] * k, v[1] * k] };
}
