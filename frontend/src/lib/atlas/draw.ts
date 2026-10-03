// ── Atlas renderer ──────────────────────────────────────────────────────────

import type * as app from "$models/app";
import { polygonOnScreen } from "./footprint";
import { drawGrid } from "./grid";
import { LABEL_FONT, LABEL_FONT_EMPHASIS, LABEL_FONT_SMALL, LabelPlacer } from "./labels";
import { project, type Viewport } from "./projection";
import { frameShape, type AtlasScene, type FrameShape } from "./scene";

/** Canvas palette — also used for the help popover's colour legend. */
export const ATLAS_COLORS = {
  frame: "rgb(100,165,255)",
  selected: "rgb(255,210,80)",
  hovered: "rgb(120,215,255)",
  star: "rgb(230,238,255)",
  dso: "rgb(255,195,90)",
} as const;

/** Frames whose footprint is smaller than this get an extra centre marker so they stay findable. */
const SMALL_FOOTPRINT_PX = 16;

const labels = new LabelPlacer();

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

export function drawScene(ctx: CanvasRenderingContext2D, scene: AtlasScene): void {
  const { vp } = scene;
  ctx.clearRect(0, 0, vp.w, vp.h);
  labels.reset();
  drawBackground(ctx, vp);
  drawGrid(ctx, vp, labels, scene.showLabels);
  // Frame labels are placed before catalog labels so they win collisions —
  // the user's own data takes priority over background star/DSO names.
  drawFrames(ctx, scene);
  drawCatalog(ctx, scene);
  drawFocus(ctx, scene);
  drawCompass(ctx, vp);
}

function drawBackground(ctx: CanvasRenderingContext2D, vp: Viewport) {
  const grad = ctx.createRadialGradient(
    vp.w / 2,
    vp.h * 0.4,
    0,
    vp.w / 2,
    vp.h * 0.4,
    Math.max(vp.w, vp.h) * 0.75,
  );
  grad.addColorStop(0, "#0c1024");
  grad.addColorStop(0.55, "#070912");
  grad.addColorStop(1, "#04050a");
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, vp.w, vp.h);

  ensureDust(vp.w, vp.h);
  for (const d of dust) {
    ctx.beginPath();
    ctx.arc(d.x, d.y, d.r, 0, Math.PI * 2);
    ctx.fillStyle = `rgba(210,220,255,${d.a})`;
    ctx.fill();
  }
}

function drawCatalog(ctx: CanvasRenderingContext2D, scene: AtlasScene) {
  const { vp } = scene;
  ctx.save();
  for (const obj of scene.catalog) {
    const p = project(vp, obj.ra, obj.dec);
    if (!p) continue;
    const [x, y] = p;
    if (x < -20 || x > vp.w + 20 || y < -20 || y > vp.h + 20) continue;

    if (obj.type === "star") {
      const r = Math.max(0.8, Math.min(3.5, (4 - obj.mag) * 0.6));
      if (obj.mag < 3.5) {
        const glow = ctx.createRadialGradient(x, y, 0, x, y, r * 4);
        glow.addColorStop(0, "rgba(200,215,255,0.35)");
        glow.addColorStop(1, "rgba(200,215,255,0)");
        ctx.fillStyle = glow;
        ctx.beginPath();
        ctx.arc(x, y, r * 4, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.beginPath();
      ctx.arc(x, y, r, 0, Math.PI * 2);
      ctx.fillStyle = "rgba(230,238,255,0.92)";
      ctx.fill();
      if (scene.showLabels && (obj.mag < 2.5 || vp.ppd > 60)) {
        labels.place(ctx, obj.name, x + r + 4, y + 4, {
          font: LABEL_FONT_SMALL,
          color: "rgba(205,218,255,0.95)",
          bg: true,
        });
      }
    } else {
      const size = vp.ppd > 20 ? 5 : 3;
      ctx.strokeStyle = "rgba(255,195,90,0.75)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(x - size, y);
      ctx.lineTo(x + size, y);
      ctx.moveTo(x, y - size);
      ctx.lineTo(x, y + size);
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(x, y, size * 1.6, 0, Math.PI * 2);
      ctx.strokeStyle = "rgba(255,195,90,0.3)";
      ctx.lineWidth = 0.8;
      ctx.stroke();
      if (scene.showLabels && vp.ppd > 8) {
        labels.place(ctx, obj.name, x + size + 4, y + 4, {
          font: LABEL_FONT_SMALL,
          color: "rgba(255,215,135,0.95)",
          bg: true,
        });
      }
    }
  }
  ctx.restore();
}

function drawFrames(ctx: CanvasRenderingContext2D, scene: AtlasScene) {
  const overlaid = new Set(scene.overlay.map((e) => e.nasPath));
  const visible = new Set(scene.frames.map((e) => e.nasPath));
  let hovered: app.AtlasIndexEntry | null = null;

  ctx.save();
  // Bottom layer: plain frames
  for (const entry of scene.frames) {
    if (overlaid.has(entry.nasPath)) continue;
    if (entry.nasPath === scene.hoveredPath) {
      hovered = entry;
      continue;
    }
    drawFrame(ctx, scene, entry, "normal");
  }
  // Overlaid frames in z-order (first = bottom, last = top)
  for (const entry of scene.overlay) {
    if (!visible.has(entry.nasPath)) continue;
    drawFrame(ctx, scene, entry, "selected");
  }
  // Hovered (non-overlaid) frame on top so its outline is never hidden
  if (hovered) drawFrame(ctx, scene, hovered, "hovered");
  ctx.restore();
}

type FrameState = "normal" | "selected" | "hovered";

function strokeFor(state: FrameState): string {
  return state === "hovered"
    ? "rgba(120,215,255,0.95)"
    : state === "selected"
      ? "rgba(255,210,80,0.85)"
      : "rgba(100,165,255,0.6)";
}

function fillFor(state: FrameState): string {
  return state === "hovered"
    ? "rgba(100,190,255,0.22)"
    : state === "selected"
      ? "rgba(255,190,70,0.12)"
      : "rgba(80,140,220,0.10)";
}

function tracePoly(ctx: CanvasRenderingContext2D, pts: [number, number][]) {
  ctx.beginPath();
  ctx.moveTo(pts[0][0], pts[0][1]);
  for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i][0], pts[i][1]);
  ctx.closePath();
}

function drawFrame(
  ctx: CanvasRenderingContext2D,
  scene: AtlasScene,
  entry: app.AtlasIndexEntry,
  state: FrameState,
) {
  const shape = frameShape(scene, entry);
  if (!shape) return;
  const { vp } = scene;
  const name = entry.object || entry.name;
  const emphasis = state !== "normal";

  if (shape.kind === "dot") {
    drawMarker(ctx, shape, state);
    if (scene.showLabels && (emphasis || vp.ppd >= 3)) {
      labels.place(ctx, name, shape.cx + 9, shape.cy + 4, {
        font: emphasis ? LABEL_FONT_EMPHASIS : LABEL_FONT_SMALL,
        color: emphasis ? "rgba(255,228,140,1)" : "rgba(195,222,255,0.95)",
        force: emphasis,
        bg: emphasis,
      });
    }
    return;
  }

  const { pts } = shape;
  if (!polygonOnScreen(pts, vp, 0)) return;

  const img = state === "selected" ? scene.preview(entry.nasPath) : undefined;
  if (img) {
    drawImageInFootprint(ctx, img, pts);
    tracePoly(ctx, pts);
    ctx.strokeStyle = strokeFor(state);
    ctx.lineWidth = 1.5;
    ctx.stroke();
    return;
  }

  const loading = state === "selected" && scene.previewLoading(entry.nasPath);
  tracePoly(ctx, pts);
  ctx.fillStyle = fillFor(state);
  ctx.fill();
  ctx.strokeStyle = strokeFor(state);
  ctx.lineWidth = state === "hovered" ? 2 : state === "selected" ? 1.8 : 1.2;
  if (loading) ctx.setLineDash([6, 4]);
  ctx.stroke();
  ctx.setLineDash([]);

  if (shape.span < SMALL_FOOTPRINT_PX) drawMarker(ctx, shape, state, 2.5);

  // Corner ticks give footprints a "finder chart" feel instead of a flat box
  if (emphasis && shape.span >= 40) {
    const tick = Math.min(14, shape.span / 6);
    ctx.strokeStyle = strokeFor(state);
    ctx.lineWidth = 2;
    for (const [vx, vy] of pts) {
      const dx = vx < shape.cx ? 1 : -1;
      const dy = vy < shape.cy ? 1 : -1;
      ctx.beginPath();
      ctx.moveTo(vx, vy + dy * tick);
      ctx.lineTo(vx, vy);
      ctx.lineTo(vx + dx * tick, vy);
      ctx.stroke();
    }
  }

  if (scene.showLabels) {
    const text = loading ? `${name} — loading…` : name;
    const big = shape.span >= 60;
    labels.place(ctx, text, big ? shape.cx : shape.cx + shape.span / 2 + 6, shape.cy + 5, {
      font: emphasis ? LABEL_FONT_EMPHASIS : LABEL_FONT,
      color:
        state === "hovered"
          ? "rgba(215,238,255,1)"
          : state === "selected"
            ? "rgba(255,228,140,1)"
            : "rgba(185,218,255,0.95)",
      align: big ? "center" : "left",
      force: emphasis,
      bg: emphasis,
    });
  }
}

function drawMarker(
  ctx: CanvasRenderingContext2D,
  shape: FrameShape,
  state: FrameState,
  radius?: number,
) {
  const r = radius ?? (state === "selected" ? 5.5 : state === "hovered" ? 5 : 4);
  const { cx, cy } = shape;
  if (state !== "normal") {
    const glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, r * 3.2);
    glow.addColorStop(0, state === "selected" ? "rgba(255,210,80,0.4)" : "rgba(120,210,255,0.4)");
    glow.addColorStop(1, "rgba(120,210,255,0)");
    ctx.fillStyle = glow;
    ctx.beginPath();
    ctx.arc(cx, cy, r * 3.2, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fillStyle =
    state === "selected"
      ? "rgba(255,210,80,0.95)"
      : state === "hovered"
        ? "rgba(120,215,255,0.95)"
        : "rgba(100,165,255,0.8)";
  ctx.fill();
  ctx.strokeStyle = "rgba(6,8,18,0.8)";
  ctx.lineWidth = 1;
  ctx.stroke();
}

/**
 * Draws the preview so its corners land exactly on the footprint corners
 * (TL, TR, BR, BL). Using the projected corners as an affine frame means the
 * image always matches its outline, including the local tilt of North away
 * from the view centre.
 */
function drawImageInFootprint(
  ctx: CanvasRenderingContext2D,
  img: HTMLCanvasElement,
  pts: [number, number][],
) {
  const [tl, tr, , bl] = pts;
  ctx.save();
  ctx.transform(
    (tr[0] - tl[0]) / img.width,
    (tr[1] - tl[1]) / img.width,
    (bl[0] - tl[0]) / img.height,
    (bl[1] - tl[1]) / img.height,
    tl[0],
    tl[1],
  );
  ctx.drawImage(img, 0, 0);
  ctx.restore();
}

function drawFocus(ctx: CanvasRenderingContext2D, scene: AtlasScene) {
  if (!scene.focus) return;
  const p = project(scene.vp, scene.focus.ra, scene.focus.dec);
  if (!p) return;
  ctx.save();
  ctx.strokeStyle = "rgba(255,195,90,0.9)";
  ctx.lineWidth = 1.5;
  ctx.setLineDash([4, 4]);
  ctx.beginPath();
  ctx.arc(p[0], p[1], 18, 0, Math.PI * 2);
  ctx.stroke();
  ctx.setLineDash([]);
  ctx.restore();
}

function drawCompass(ctx: CanvasRenderingContext2D, vp: Viewport) {
  // Bottom-left; North is up and East is LEFT (sky as seen from the ground).
  const cx = 36;
  const cy = vp.h - 40;
  const r = 22;
  ctx.save();
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fillStyle = "rgba(10,12,24,0.7)";
  ctx.fill();
  ctx.strokeStyle = "rgba(110,140,195,0.5)";
  ctx.lineWidth = 1;
  ctx.stroke();

  ctx.strokeStyle = "rgba(150,180,235,0.75)";
  ctx.lineWidth = 1.3;
  ctx.beginPath();
  ctx.moveTo(cx, cy - r + 6);
  ctx.lineTo(cx, cy + r - 6);
  ctx.moveTo(cx - r + 6, cy);
  ctx.lineTo(cx + r - 6, cy);
  ctx.stroke();

  ctx.font = LABEL_FONT_EMPHASIS;
  ctx.textAlign = "center";
  ctx.fillStyle = "rgba(205,222,255,1)";
  ctx.fillText("N", cx, cy - r + 14);
  ctx.fillText("E", cx - r + 10, cy + 5);
  ctx.textAlign = "left";
  ctx.restore();
}
