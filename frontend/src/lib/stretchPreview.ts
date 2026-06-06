import type * as fits from "$models/fits";

export function mtfMidtone(target: number, x: number): number {
  if (x === 0) return 0;
  const d = x * (1 - 2 * target) + target;
  if (d === 0) return 0;
  return (x * (1 - target)) / d;
}

function mtf(m: number, x: number): number {
  if (x <= 0) return 0;
  if (x >= 1) return 1;
  if (m <= 0) return 0;
  if (m >= 1) return 1;
  return ((m - 1) * x) / ((2 * m - 1) * x - m);
}

interface ChannelParams {
  shadows: number;
  midtone: number;
  linear: boolean;
}

function computeParams(stats: fits.ChannelStats[], stretchLevel: number): ChannelParams[] {
  const presets = [
    { shadowsFactor: -1.25, targetBG: 0.1 },
    { shadowsFactor: -2.8, targetBG: 0.25 },
    { shadowsFactor: -4.0, targetBG: 0.4 },
  ];

  if (stretchLevel <= 0) {
    return stats.map((s) => ({
      shadows: Math.max(0, s.median - 2.8 * s.sigma),
      midtone: 0.5,
      linear: true,
    }));
  }

  const preset = presets[Math.max(0, Math.min(stretchLevel - 1, presets.length - 1))];
  return stats.map((s) => {
    const shadows = Math.max(0, s.median + preset.shadowsFactor * s.sigma);
    const scale = 1 - shadows;
    if (scale <= 0) return { shadows: 0, midtone: 0.25, linear: false };
    const newMedian = Math.max(0, s.median - shadows) / scale;
    return { shadows, midtone: mtfMidtone(preset.targetBG, newMedian), linear: false };
  });
}

function applyStretch(p: ChannelParams, v: number): number {
  const scale = 1 - p.shadows;
  if (scale <= 0) return 0;
  const x = Math.max(0, Math.min(1, (v - p.shadows) / scale));
  if (p.linear) return x;
  return Math.max(0, Math.min(1, mtf(p.midtone, x)));
}

/**
 * Applies MTF stretch to raw float32 RGBA data and returns a 2D canvas.
 * Input f32 values are in [0,1]. Works for both mono (channels=1) and color.
 */
export function renderStretched(
  f32: Float32Array,
  width: number,
  height: number,
  channels: number,
  stats: fits.ChannelStats[],
  stretchLevel: number,
): HTMLCanvasElement {
  const params = computeParams(stats, stretchLevel);
  const p0 = params[0] ?? { shadows: 0, midtone: 0.5, linear: true };
  const p1 = params[1] ?? p0;
  const p2 = params[2] ?? p0;

  const c = document.createElement("canvas");
  c.width = width;
  c.height = height;
  const ctx = c.getContext("2d")!;
  const img = ctx.createImageData(width, height);
  const out = img.data;

  for (let i = 0, o = 0; i < f32.length; i += 4, o += 4) {
    if (channels === 1) {
      const v = Math.round(applyStretch(p0, f32[i]) * 255);
      out[o] = v;
      out[o + 1] = v;
      out[o + 2] = v;
    } else {
      out[o] = Math.round(applyStretch(p0, f32[i]) * 255);
      out[o + 1] = Math.round(applyStretch(p1, f32[i + 1]) * 255);
      out[o + 2] = Math.round(applyStretch(p2, f32[i + 2]) * 255);
    }
    out[o + 3] = 255;
  }

  ctx.putImageData(img, 0, 0);
  return c;
}
