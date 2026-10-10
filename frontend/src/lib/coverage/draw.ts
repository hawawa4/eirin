// ── Coverage renderer ───────────────────────────────────────────────────────
// Sky layers from the atlas, with each cluster drawn as its scope-coloured
// outline: dashed when the position is estimated, gold when selected.

import type * as app from "$models/app";
import { polygonOnScreen } from "../atlas/footprint";
import { drawGrid } from "../atlas/grid";
import { LABEL_FONT, LABEL_FONT_EMPHASIS, LabelPlacer } from "../atlas/labels";
import { rgba, type AtlasPalette, type RGB } from "../atlas/palette";
import { drawBackground, drawCatalog, drawCompass, drawFocus } from "../atlas/sky";
import { clusterShape, type ClusterShape, type CoverageScene } from "./scene";
import { clusterName } from "./selection";

type ClusterState = "normal" | "selected" | "hovered";

/** Outlines at least this big (px) get their label centred inside. */
const INNER_LABEL_PX = 70;

const labels = new LabelPlacer();

export function drawCoverage(ctx: CanvasRenderingContext2D, scene: CoverageScene): void {
  const { vp, palette: pal } = scene;
  ctx.clearRect(0, 0, vp.w, vp.h);
  labels.reset(rgba(pal.ink, 0.82));
  drawBackground(ctx, vp, pal);
  drawGrid(ctx, vp, labels, scene.showLabels, pal);
  drawClusters(ctx, scene);
  drawCatalog(ctx, vp, pal, labels, scene.catalog, scene.showLabels);
  drawFocus(ctx, vp, pal, scene.focus);
  drawCompass(ctx, vp, pal);
}

interface Placed {
  c: app.CoverageCluster;
  shape: ClusterShape;
  state: ClusterState;
  selected: boolean;
}

function drawClusters(ctx: CanvasRenderingContext2D, scene: CoverageScene) {
  const placed: Placed[] = [];
  for (const c of scene.clusters) {
    const shape = clusterShape(scene.vp, c);
    if (!shape) continue;
    if (shape.kind === "poly" && !polygonOnScreen(shape.pts, scene.vp, 0)) continue;
    const selected = scene.selected.has(c.id);
    const state: ClusterState =
      c.id === scene.hoveredId ? "hovered" : selected ? "selected" : "normal";
    placed.push({ c, shape, state, selected });
  }
  // Big outlines first so smaller ones stay visible on top; then the
  // selection, then the hovered cluster. Labels go in reverse priority order.
  const rank = (p: Placed) => (p.state === "hovered" ? 2 : p.state === "selected" ? 1 : 0);
  const area = (p: Placed) => (p.shape.kind === "poly" ? p.shape.area : 0);
  placed.sort((a, b) => rank(a) - rank(b) || area(b) - area(a));

  ctx.save();
  for (const p of placed) drawShape(ctx, scene, p);
  if (scene.showLabels) {
    for (let i = placed.length - 1; i >= 0; i--) drawLabel(ctx, scene, placed[i]);
  }
  ctx.restore();
}

function colourOf(scene: CoverageScene, p: Placed): RGB {
  return p.selected ? scene.palette.selected : scene.scopeColor(p.c.scope);
}

function drawShape(ctx: CanvasRenderingContext2D, scene: CoverageScene, p: Placed) {
  const col = colourOf(scene, p);
  const { shape, state } = p;

  if (shape.kind === "dot") {
    drawMarker(ctx, scene.palette, shape.cx, shape.cy, col, state);
    return;
  }

  ctx.beginPath();
  ctx.moveTo(shape.pts[0][0], shape.pts[0][1]);
  for (let i = 1; i < shape.pts.length; i++) ctx.lineTo(shape.pts[i][0], shape.pts[i][1]);
  ctx.closePath();
  ctx.fillStyle = rgba(col, state === "hovered" ? 0.2 : state === "selected" ? 0.16 : 0.07);
  ctx.fill();
  ctx.strokeStyle = rgba(col, state === "normal" ? 0.75 : 0.95);
  ctx.lineWidth = state === "normal" ? 1.2 : 2;
  if (p.c.approx) ctx.setLineDash([6, 4]);
  ctx.stroke();
  ctx.setLineDash([]);
}

function drawMarker(
  ctx: CanvasRenderingContext2D,
  pal: AtlasPalette,
  cx: number,
  cy: number,
  col: RGB,
  state: ClusterState,
) {
  const r = state === "normal" ? 4 : 5.5;
  if (state !== "normal") {
    const glow = ctx.createRadialGradient(cx, cy, 0, cx, cy, r * 3.2);
    glow.addColorStop(0, rgba(col, 0.4));
    glow.addColorStop(1, rgba(col, 0));
    ctx.fillStyle = glow;
    ctx.beginPath();
    ctx.arc(cx, cy, r * 3.2, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.beginPath();
  ctx.arc(cx, cy, r, 0, Math.PI * 2);
  ctx.fillStyle = rgba(col, 0.9);
  ctx.fill();
  ctx.strokeStyle = rgba(pal.ink, 0.8);
  ctx.lineWidth = 1;
  ctx.stroke();
}

function drawLabel(ctx: CanvasRenderingContext2D, scene: CoverageScene, p: Placed) {
  const { shape, state, c } = p;
  const emphasis = state !== "normal";
  if (!emphasis && shape.kind === "dot" && scene.vp.ppd < 3) return;
  const pal = scene.palette;
  const color = p.selected
    ? rgba(pal.selectedLabel)
    : state === "hovered"
      ? rgba(pal.hoveredLabel)
      : rgba(pal.frameLabel, 0.95);
  const opts = {
    font: emphasis ? LABEL_FONT_EMPHASIS : LABEL_FONT,
    color,
    force: emphasis,
    bg: emphasis,
  };
  const text = clusterName(c);
  if (shape.kind === "poly" && shape.span >= INNER_LABEL_PX) {
    labels.place(ctx, text, shape.cx, shape.cy + 5, { ...opts, align: "center" });
  } else {
    const dx = shape.kind === "poly" ? shape.span / 2 + 6 : 9;
    labels.place(ctx, text, shape.cx + dx, shape.cy + 4, opts);
  }
}
