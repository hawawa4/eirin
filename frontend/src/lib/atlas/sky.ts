// ── Sky layers shared by the Sky Atlas and Coverage views ───────────────────
// Background, catalog stars/DSOs, focus ring and compass. Each view draws its
// own frames between these layers.

import type * as app from "$models/app";
import { LABEL_FONT_EMPHASIS, LABEL_FONT_SMALL, type LabelPlacer } from "./labels";
import { project, type Viewport } from "./projection";
import { rgba, type AtlasPalette } from "./palette";

// Fixed background dust — decorative only, regenerated on resize.
let dust: { x: number; y: number; r: number; a: number }[] = [];
let dustW = 0;
let dustH = 0;

function ensureDust(w: number, h: number) {
  if (dustW === w && dustH === h && dust.length) return;
  dustW = w;
  dustH = h;
  const count = Math.min(400, Math.round((w * h) / 2200));
  // Deterministic PRNG so the field doesn't jump around on every resize tick.
  let seed = 1337;
  const rand = () => {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff;
    return seed / 0x7fffffff;
  };
  dust = [];
  for (let i = 0; i < count; i++) {
    dust.push({ x: rand() * w, y: rand() * h, r: rand() * 1.1 + 0.2, a: rand() * 0.5 + 0.15 });
  }
}

export function drawBackground(ctx: CanvasRenderingContext2D, vp: Viewport, pal: AtlasPalette) {
  const grad = ctx.createRadialGradient(
    vp.w / 2,
    vp.h * 0.4,
    0,
    vp.w / 2,
    vp.h * 0.4,
    Math.max(vp.w, vp.h) * 0.75,
  );
  grad.addColorStop(0, pal.bg[0]);
  grad.addColorStop(0.55, pal.bg[1]);
  grad.addColorStop(1, pal.bg[2]);
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, vp.w, vp.h);

  ensureDust(vp.w, vp.h);
  for (const d of dust) {
    ctx.beginPath();
    ctx.arc(d.x, d.y, d.r, 0, Math.PI * 2);
    ctx.fillStyle = rgba(pal.dust, d.a);
    ctx.fill();
  }
}

export function drawCatalog(
  ctx: CanvasRenderingContext2D,
  vp: Viewport,
  pal: AtlasPalette,
  labels: LabelPlacer,
  catalog: readonly app.CatalogObject[],
  showLabels: boolean,
) {
  ctx.save();
  for (const obj of catalog) {
    const p = project(vp, obj.ra, obj.dec);
    if (!p) continue;
    const [x, y] = p;
    if (x < -20 || x > vp.w + 20 || y < -20 || y > vp.h + 20) continue;

    if (obj.type === "star") {
      const r = Math.max(0.8, Math.min(3.5, (4 - obj.mag) * 0.6));
      if (obj.mag < 3.5) {
        const glow = ctx.createRadialGradient(x, y, 0, x, y, r * 4);
        glow.addColorStop(0, rgba(pal.star, 0.35));
        glow.addColorStop(1, rgba(pal.star, 0));
        ctx.fillStyle = glow;
        ctx.beginPath();
        ctx.arc(x, y, r * 4, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.beginPath();
      ctx.arc(x, y, r, 0, Math.PI * 2);
      ctx.fillStyle = rgba(pal.star, 0.92);
      ctx.fill();
      if (showLabels && (obj.mag < 2.5 || vp.ppd > 60)) {
        labels.place(ctx, obj.name, x + r + 4, y + 4, {
          font: LABEL_FONT_SMALL,
          color: rgba(pal.starLabel, 0.95),
          bg: true,
        });
      }
    } else {
      const size = vp.ppd > 20 ? 5 : 3;
      ctx.strokeStyle = rgba(pal.dso, 0.75);
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(x - size, y);
      ctx.lineTo(x + size, y);
      ctx.moveTo(x, y - size);
      ctx.lineTo(x, y + size);
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(x, y, size * 1.6, 0, Math.PI * 2);
      ctx.strokeStyle = rgba(pal.dso, 0.3);
      ctx.lineWidth = 0.8;
      ctx.stroke();
      if (showLabels && vp.ppd > 8) {
        labels.place(ctx, obj.name, x + size + 4, y + 4, {
          font: LABEL_FONT_SMALL,
          color: rgba(pal.dsoLabel, 0.95),
          bg: true,
        });
      }
    }
  }
  ctx.restore();
}

/** Dashed target ring after flying to a catalog object. */
export function drawFocus(
  ctx: CanvasRenderingContext2D,
  vp: Viewport,
  pal: AtlasPalette,
  focus: { ra: number; dec: number } | null,
) {
  if (!focus) return;
  const p = project(vp, focus.ra, focus.dec);
  if (!p) return;
  ctx.save();
  ctx.strokeStyle = rgba(pal.dso, 0.9);
  ctx.lineWidth = 1.5;
  ctx.setLineDash([4, 4]);
  ctx.beginPath();
  ctx.arc(p[0], p[1], 18, 0, Math.PI * 2);
  ctx.stroke();
  ctx.setLineDash([]);
  ctx.restore();
}

export function drawCompass(ctx: CanvasRenderingContext2D, vp: Viewport, pal: AtlasPalette) {
  // Bottom-left; North is up and East is LEFT (sky as seen from the ground).
  const cx = 36;
  const cy = vp.h - 40;
  const r = 22;
  ctx.save();
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fillStyle = rgba(pal.ink, 0.7);
  ctx.fill();
  ctx.strokeStyle = rgba(pal.compass, 0.45);
  ctx.lineWidth = 1;
  ctx.stroke();

  ctx.strokeStyle = rgba(pal.compass, 0.75);
  ctx.lineWidth = 1.3;
  ctx.beginPath();
  ctx.moveTo(cx, cy - r + 6);
  ctx.lineTo(cx, cy + r - 6);
  ctx.moveTo(cx - r + 6, cy);
  ctx.lineTo(cx + r - 6, cy);
  ctx.stroke();

  ctx.font = LABEL_FONT_EMPHASIS;
  ctx.textAlign = "center";
  ctx.fillStyle = rgba(pal.compassText);
  ctx.fillText("N", cx, cy - r + 14);
  ctx.fillText("E", cx - r + 10, cy + 5);
  ctx.textAlign = "left";
  ctx.restore();
}
