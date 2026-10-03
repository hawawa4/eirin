// ── Atlas scene: what's on screen, where, and what's under the pointer ──────

import type * as app from "$models/app";
import {
  footprintCorners,
  frameExtentDeg,
  pointInPolygon,
  polygonArea,
  polygonSpan,
  type FrameSize,
} from "./footprint";
import { project, type Viewport } from "./projection";

/** Footprints smaller than this (canvas px, long side) are drawn as markers instead. */
export const MIN_FOOTPRINT_PX = 6;
/** Pointer distance (px) from a frame centre that counts as hovering it. */
export const HIT_RADIUS_PX = 10;
/** Max frames whose size is requested per view change. */
const MAX_SIZE_REQUESTS = 400;

/** Everything the renderer and hit-tester need to know about the current state. */
export interface AtlasScene {
  vp: Viewport;
  /** Frames passing the current filter. */
  frames: app.AtlasIndexEntry[];
  /** Overlaid frames in z-order (last = top, shown in the panel). */
  overlay: app.AtlasIndexEntry[];
  hoveredPath: string | null;
  catalog: app.CatalogObject[];
  showLabels: boolean;
  /** Pulsing target ring after flying to a catalog object. */
  focus: { ra: number; dec: number } | null;
  size(path: string): FrameSize | undefined;
  preview(path: string): HTMLCanvasElement | undefined;
  previewLoading(path: string): boolean;
  /** Effective rotation (stored rotation + any user orientation fix). */
  rotation(entry: app.AtlasIndexEntry): number;
}

export type FrameShape =
  | { kind: "poly"; cx: number; cy: number; pts: [number, number][]; span: number }
  | { kind: "dot"; cx: number; cy: number };

/** Screen geometry for a frame: a footprint polygon when big enough, else a marker. */
export function frameShape(scene: AtlasScene, entry: app.AtlasIndexEntry): FrameShape | null {
  const c = project(scene.vp, entry.ra, entry.dec);
  if (!c) return null;
  const sz = scene.size(entry.nasPath);
  if (sz) {
    const pts = footprintCorners(scene.vp, entry, sz, scene.rotation(entry));
    if (pts) {
      const span = polygonSpan(pts);
      if (span >= MIN_FOOTPRINT_PX) return { kind: "poly", cx: c[0], cy: c[1], pts, span };
    }
  }
  return { kind: "dot", cx: c[0], cy: c[1] };
}

/**
 * The frame under (x, y): the nearest frame centre within HIT_RADIUS_PX wins
 * (so small frames inside large mosaics stay clickable); otherwise the
 * smallest footprint containing the point.
 */
export function hitTest(scene: AtlasScene, x: number, y: number): app.AtlasIndexEntry | null {
  let bestDot: app.AtlasIndexEntry | null = null;
  let bestDist = HIT_RADIUS_PX;
  let bestPoly: app.AtlasIndexEntry | null = null;
  let bestArea = Infinity;
  for (const entry of scene.frames) {
    const s = frameShape(scene, entry);
    if (!s) continue;
    const d = Math.hypot(x - s.cx, y - s.cy);
    if (d <= bestDist) {
      bestDist = d;
      bestDot = entry;
    }
    if (!bestDot && s.kind === "poly" && pointInPolygon(x, y, s.pts)) {
      const area = polygonArea(s.pts);
      if (area < bestArea) {
        bestArea = area;
        bestPoly = entry;
      }
    }
  }
  return bestDot ?? bestPoly;
}

/**
 * Paths of frames near the viewport whose footprint would be visible
 * (≥ MIN_FOOTPRINT_PX) but whose pixel size isn't known yet, nearest-to-centre first.
 */
export function framesNeedingSize(scene: AtlasScene): string[] {
  const { vp } = scene;
  const margin = 200;
  const out: { path: string; d: number }[] = [];
  for (const entry of scene.frames) {
    if (scene.size(entry.nasPath)) continue;
    if (frameExtentDeg(entry, undefined) * vp.ppd < MIN_FOOTPRINT_PX) continue;
    const p = project(vp, entry.ra, entry.dec);
    if (!p) continue;
    const [x, y] = p;
    if (x < -margin || x > vp.w + margin || y < -margin || y > vp.h + margin) continue;
    out.push({ path: entry.nasPath, d: Math.hypot(x - vp.w / 2, y - vp.h / 2) });
  }
  out.sort((a, b) => a.d - b.d);
  return out.slice(0, MAX_SIZE_REQUESTS).map((o) => o.path);
}
