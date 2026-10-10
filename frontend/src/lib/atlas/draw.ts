// ── Atlas renderer ──────────────────────────────────────────────────────────

import type * as app from "$models/app";
import { polygonOnScreen } from "./footprint";
import { drawGrid } from "./grid";
import { LABEL_FONT, LABEL_FONT_EMPHASIS, LABEL_FONT_SMALL, LabelPlacer } from "./labels";
import { drawBackground, drawCatalog, drawCompass, drawFocus } from "./sky";
import { rgba, type AtlasPalette } from "./palette";
import { frameShape, type AtlasScene, type FrameShape } from "./scene";

/** Frames whose footprint is smaller than this get an extra centre marker so they stay findable. */
const SMALL_FOOTPRINT_PX = 16;

const labels = new LabelPlacer();

export function drawScene(ctx: CanvasRenderingContext2D, scene: AtlasScene): void {
  const { vp, palette: pal } = scene;
  ctx.clearRect(0, 0, vp.w, vp.h);
  labels.reset(rgba(pal.ink, 0.82));
  drawBackground(ctx, vp, pal);
  drawGrid(ctx, vp, labels, scene.showLabels, pal);
  // Frame labels are placed before catalog labels so they win collisions —
  // the user's own data takes priority over background star/DSO names.
  drawFrames(ctx, scene);
  drawCatalog(ctx, vp, pal, labels, scene.catalog, scene.showLabels);
  drawFocus(ctx, vp, pal, scene.focus);
  drawCompass(ctx, vp, pal);
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

function stateColor(pal: AtlasPalette, state: FrameState) {
  return state === "hovered" ? pal.hovered : state === "selected" ? pal.selected : pal.frame;
}

function strokeFor(pal: AtlasPalette, state: FrameState): string {
  const a = state === "hovered" ? 0.95 : state === "selected" ? 0.85 : 0.6;
  return rgba(stateColor(pal, state), a);
}

function fillFor(pal: AtlasPalette, state: FrameState): string {
  const a = state === "hovered" ? 0.22 : state === "selected" ? 0.12 : 0.1;
  return rgba(stateColor(pal, state), a);
}

function labelFor(pal: AtlasPalette, state: FrameState): string {
  return state === "hovered"
    ? rgba(pal.hoveredLabel)
    : state === "selected"
      ? rgba(pal.selectedLabel)
      : rgba(pal.frameLabel, 0.95);
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
  const { vp, palette: pal } = scene;
  const name = entry.object || entry.name;
  const emphasis = state !== "normal";

  if (shape.kind === "dot") {
    drawMarker(ctx, pal, shape, state);
    if (scene.showLabels && (emphasis || vp.ppd >= 3)) {
      labels.place(ctx, name, shape.cx + 9, shape.cy + 4, {
        font: emphasis ? LABEL_FONT_EMPHASIS : LABEL_FONT_SMALL,
        color: labelFor(pal, emphasis ? "selected" : "normal"),
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
    ctx.strokeStyle = strokeFor(pal, state);
    ctx.lineWidth = 1.5;
    ctx.stroke();
    return;
  }

  const loading = state === "selected" && scene.previewLoading(entry.nasPath);
  tracePoly(ctx, pts);
  ctx.fillStyle = fillFor(pal, state);
  ctx.fill();
  ctx.strokeStyle = strokeFor(pal, state);
  ctx.lineWidth = state === "hovered" ? 2 : state === "selected" ? 1.8 : 1.2;
  if (loading) ctx.setLineDash([6, 4]);
  ctx.stroke();
  ctx.setLineDash([]);

  if (shape.span < SMALL_FOOTPRINT_PX) drawMarker(ctx, pal, shape, state, 2.5);

  // Corner ticks give footprints a "finder chart" feel instead of a flat box
  if (emphasis && shape.span >= 40) {
    const tick = Math.min(14, shape.span / 6);
    ctx.strokeStyle = strokeFor(pal, state);
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
      color: labelFor(pal, state),
      align: big ? "center" : "left",
      force: emphasis,
      bg: emphasis,
    });
  }
}

function drawMarker(
  ctx: CanvasRenderingContext2D,
  pal: AtlasPalette,
  shape: FrameShape,
  state: FrameState,
  radius?: number,
) {
  const r = radius ?? (state === "selected" ? 5.5 : state === "hovered" ? 5 : 4);
  const { cx, cy } = shape;
  if (state !== "normal") {
    const glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, r * 3.2);
    const c = stateColor(pal, state);
    glow.addColorStop(0, rgba(c, 0.4));
    glow.addColorStop(1, rgba(c, 0));
    ctx.fillStyle = glow;
    ctx.beginPath();
    ctx.arc(cx, cy, r * 3.2, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fillStyle = rgba(stateColor(pal, state), state === "normal" ? 0.8 : 0.95);
  ctx.fill();
  ctx.strokeStyle = rgba(pal.ink, 0.8);
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
