import type * as fits from "$models/fits";

// ── Linked autostretch ──────────────────────────────────────────────────────
// Siril/PixInsight-style MTF autostretch, *linked*: one black point and one
// midtone for every channel, applied after per-channel balance gains that
// neutralise the sky background (computed by the backend). Stretching each
// channel on its own noise level instead tints every gradient with the
// cleanest channel — the "everything is green / red" look of raw OSC subs.
// Keep in sync with internal/fits/stretch.go.

/** Index = stretch level; level 0 is linear (black point only, raw colour). */
const PRESETS = [
  { shadows: -2.8, targetBG: 0 },
  { shadows: -1.25, targetBG: 0.1 },
  { shadows: -2.8, targetBG: 0.25 },
  { shadows: -4.0, targetBG: 0.4 },
];

export interface StretchParams {
  /** Per-channel multipliers applied before the stretch (all 1 when linear). */
  gains: [number, number, number];
  shadows: number;
  midtone: number;
  linear: boolean;
}

export function mtfMidtone(target: number, x: number): number {
  if (x === 0) return 0;
  const d = x * (1 - 2 * target) + target;
  if (d === 0) return 0;
  return (x * (1 - target)) / d;
}

export function mtf(m: number, x: number): number {
  if (x <= 0) return 0;
  if (x >= 1) return 1;
  if (m <= 0) return 0;
  if (m >= 1) return 1;
  return ((m - 1) * x) / ((2 * m - 1) * x - m);
}

/**
 * Stretch for `level` (0 = linear … 3 = strong) from per-channel stats and
 * the backend's background-balance gains (ignored when linear).
 */
export function computeStretch(
  stats: fits.ChannelStats[],
  balance: number[] | null | undefined,
  level: number,
): StretchParams {
  const lvl = Math.max(0, Math.min(PRESETS.length - 1, Math.round(level)));
  const preset = PRESETS[lvl];
  const gain = (i: number) => (lvl > 0 && balance && balance[i] > 0 ? balance[i] : 1);
  const gains: [number, number, number] = [gain(0), gain(1), gain(2)];
  if (stats.length === 0) return { gains, shadows: 0, midtone: 0.5, linear: true };

  let med = 0;
  let c0 = 0;
  stats.forEach((s, i) => {
    const m = s.median * gains[Math.min(i, 2)];
    med += m;
    c0 += m + preset.shadows * s.sigma * gains[Math.min(i, 2)];
  });
  med /= stats.length;
  c0 = Math.max(0, c0 / stats.length);

  // Background at or above white: nothing sensible to show, map it all to black.
  if (c0 >= 1) return { gains, shadows: 1, midtone: 0.5, linear: true };
  if (lvl === 0) return { gains, shadows: c0, midtone: 0.5, linear: true };
  const midtone = mtfMidtone(preset.targetBG, Math.max(0, med - c0) / (1 - c0));
  return { gains, shadows: c0, midtone, linear: false };
}

/** Stretches value `v` of channel `c` to [0,1]. */
export function applyStretch(p: StretchParams, c: number, v: number): number {
  if (p.shadows >= 1) return 0;
  const x = Math.max(0, Math.min(1, (v * p.gains[c] - p.shadows) / (1 - p.shadows)));
  return p.linear ? x : mtf(p.midtone, x);
}

/**
 * Applies the linked stretch to raw float32 RGBA data and returns a 2D canvas.
 * Input f32 values are in [0,1]. Works for both mono (channels=1) and color.
 */
export function renderStretched(
  f32: Float32Array,
  width: number,
  height: number,
  channels: number,
  stats: fits.ChannelStats[],
  balance: number[] | null | undefined,
  stretchLevel: number,
): HTMLCanvasElement {
  const p = computeStretch(stats, balance, stretchLevel);

  const c = document.createElement("canvas");
  c.width = width;
  c.height = height;
  const ctx = c.getContext("2d")!;
  const img = ctx.createImageData(width, height);
  const out = img.data;

  for (let i = 0, o = 0; i < f32.length; i += 4, o += 4) {
    if (channels === 1) {
      const v = Math.round(applyStretch(p, 0, f32[i]) * 255);
      out[o] = v;
      out[o + 1] = v;
      out[o + 2] = v;
    } else {
      out[o] = Math.round(applyStretch(p, 0, f32[i]) * 255);
      out[o + 1] = Math.round(applyStretch(p, 1, f32[i + 1]) * 255);
      out[o + 2] = Math.round(applyStretch(p, 2, f32[i + 2]) * 255);
    }
    out[o + 3] = 255;
  }

  ctx.putImageData(img, 0, 0);
  return c;
}
