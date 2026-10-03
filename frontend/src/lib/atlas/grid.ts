// ── RA/Dec coordinate grid ──────────────────────────────────────────────────
// Line spacing adapts to zoom; only lines crossing the visible region are
// sampled, so the grid stays cheap at any zoom level.

import { LABEL_FONT_SMALL, type LabelPlacer } from "./labels";
import { DEG, normRA, project, unproject, type Viewport } from "./projection";

/** Candidate RA steps in degrees (multiples of time units: 2h, 1h, 30m, 20m, 10m, 5m, 2m, 1m, 30s). */
const RA_STEPS = [30, 15, 7.5, 5, 2.5, 1.25, 0.5, 0.25, 0.125];
/** Candidate Dec steps in degrees (30°, 10°, 5°, 2°, 1°, 30′, 15′, 10′, 5′). */
const DEC_STEPS = [30, 10, 5, 2, 1, 0.5, 0.25, 1 / 6, 1 / 12];
const MIN_LINE_SPACING_PX = 90;
const SAMPLES = 64;
const MAX_MERIDIANS = 24;

function pickStep(steps: number[], pxPerUnit: number): number {
  for (let i = steps.length - 1; i >= 0; i--) {
    if (steps[i] * pxPerUnit >= MIN_LINE_SPACING_PX) return steps[i];
  }
  return steps[0];
}

/** Wraps an RA difference into (-180, 180]. */
function wrap180(d: number): number {
  return ((((d + 180) % 360) + 360) % 360) - 180;
}

/** Visible RA/Dec ranges (RA as offsets from the view centre). */
function visibleRange(vp: Viewport): { raLo: number; raHi: number; decLo: number; decHi: number } {
  let raLo = Infinity;
  let raHi = -Infinity;
  let decLo = Infinity;
  let decHi = -Infinity;
  const n = 6;
  for (let i = 0; i <= n; i++) {
    for (let j = 0; j <= n; j++) {
      const [ra, dec] = unproject(vp, (vp.w * i) / n, (vp.h * j) / n);
      const d = wrap180(ra - vp.ra);
      raLo = Math.min(raLo, d);
      raHi = Math.max(raHi, d);
      decLo = Math.min(decLo, dec);
      decHi = Math.max(decHi, dec);
    }
  }
  // A visible pole means every RA is on screen.
  const inView = (p: [number, number] | null) =>
    p !== null && p[0] >= 0 && p[0] <= vp.w && p[1] >= 0 && p[1] <= vp.h;
  if (inView(project(vp, 0, 90))) {
    decHi = 90;
    raLo = -180;
    raHi = 180;
  }
  if (inView(project(vp, 0, -90))) {
    decLo = -90;
    raLo = -180;
    raHi = 180;
  }
  return { raLo, raHi, decLo, decHi };
}

function strokePolyline(ctx: CanvasRenderingContext2D, pts: ([number, number] | null)[]) {
  ctx.beginPath();
  let pen = false;
  for (const p of pts) {
    if (!p) {
      pen = false;
      continue;
    }
    if (pen) ctx.lineTo(p[0], p[1]);
    else ctx.moveTo(p[0], p[1]);
    pen = true;
  }
  ctx.stroke();
}

function fmtRA(ra: number, step: number): string {
  const totalSec = Math.round((normRA(ra) / 15) * 3600);
  const h = Math.floor(totalSec / 3600) % 24;
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (step >= 15) return `${h}h`;
  if (step >= 0.25) return `${h}h${String(m).padStart(2, "0")}m`;
  return `${h}h${String(m).padStart(2, "0")}m${String(s).padStart(2, "0")}s`;
}

function fmtDec(dec: number, step: number): string {
  const sign = dec > 0 ? "+" : dec < 0 ? "−" : "";
  const abs = Math.abs(dec);
  if (step >= 1) return `${sign}${Math.round(abs)}°`;
  const totalMin = Math.round(abs * 60);
  return `${sign}${Math.floor(totalMin / 60)}°${String(totalMin % 60).padStart(2, "0")}′`;
}

export function drawGrid(
  ctx: CanvasRenderingContext2D,
  vp: Viewport,
  labels: LabelPlacer,
  showLabels: boolean,
): void {
  const r = visibleRange(vp);
  const cosDec = Math.max(0.05, Math.cos(vp.dec * DEG));
  // Near a pole every meridian converges on screen: cap the meridian count.
  let raIdx = RA_STEPS.indexOf(pickStep(RA_STEPS, vp.ppd * cosDec));
  while (raIdx > 0 && (r.raHi - r.raLo) / RA_STEPS[raIdx] > MAX_MERIDIANS) raIdx--;
  const raStep = RA_STEPS[raIdx];
  const decStep = pickStep(DEC_STEPS, vp.ppd);
  const raLo = Math.max(-180, r.raLo - raStep);
  const raHi = Math.min(180, r.raHi + raStep);
  const decLo = Math.max(-90, r.decLo - decStep);
  const decHi = Math.min(90, r.decHi + decStep);

  ctx.save();
  ctx.lineWidth = 1;
  ctx.setLineDash([1, 5]);
  ctx.lineCap = "round";

  // Parallels (constant Dec)
  const decStart = Math.ceil(decLo / decStep) * decStep;
  for (let dec = decStart; dec <= decHi + 1e-9; dec += decStep) {
    if (Math.abs(dec) >= 90) continue;
    const pts: ([number, number] | null)[] = [];
    for (let i = 0; i <= SAMPLES; i++) {
      pts.push(project(vp, vp.ra + raLo + ((raHi - raLo) * i) / SAMPLES, dec));
    }
    ctx.strokeStyle = Math.abs(dec) < 1e-9 ? "rgba(130,155,210,0.5)" : "rgba(90,115,165,0.34)";
    strokePolyline(ctx, pts);
  }

  // Meridians (constant RA), aligned to absolute RA multiples of the step
  const raAbsStart = Math.ceil((vp.ra + raLo) / raStep) * raStep;
  const meridians: number[] = [];
  for (let ra = raAbsStart; ra <= vp.ra + raHi + 1e-9; ra += raStep) meridians.push(ra);
  for (const ra of meridians) {
    const pts: ([number, number] | null)[] = [];
    for (let i = 0; i <= SAMPLES; i++) {
      pts.push(
        project(vp, ra, Math.max(-89.5, Math.min(89.5, decLo + ((decHi - decLo) * i) / SAMPLES))),
      );
    }
    const isZero = Math.abs(wrap180(ra)) < 1e-6;
    ctx.strokeStyle = isZero ? "rgba(130,155,210,0.5)" : "rgba(90,115,165,0.34)";
    strokePolyline(ctx, pts);
  }
  ctx.setLineDash([]);

  if (showLabels) {
    // RA labels along the current Dec, Dec labels along the current RA.
    for (const ra of meridians) {
      const lp = project(vp, ra, vp.dec);
      if (lp && lp[0] >= 20 && lp[0] <= vp.w - 40 && lp[1] >= 16 && lp[1] <= vp.h - 4) {
        labels.place(ctx, fmtRA(ra, raStep), lp[0] + 5, lp[1] - 3, {
          font: LABEL_FONT_SMALL,
          color: "rgba(170,195,240,0.95)",
          bg: true,
        });
      }
    }
    for (let dec = decStart; dec <= decHi + 1e-9; dec += decStep) {
      if (Math.abs(dec) >= 90) continue;
      const lp = project(vp, vp.ra, dec);
      if (lp && lp[1] >= 16 && lp[1] <= vp.h - 4 && lp[0] >= 4 && lp[0] <= vp.w - 40) {
        labels.place(ctx, fmtDec(dec, decStep), lp[0] + 6, lp[1] - 3, {
          font: LABEL_FONT_SMALL,
          color: "rgba(130,225,200,0.95)",
          bg: true,
        });
      }
    }
  }
  ctx.restore();
}
