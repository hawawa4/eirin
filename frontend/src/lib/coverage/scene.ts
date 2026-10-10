// ── Coverage scene: cluster outlines on screen and what's under the pointer ──

import type * as app from "$models/app";
import { pointInPolygon, polygonArea, polygonSpan } from "../atlas/footprint";
import type { AtlasPalette, RGB } from "../atlas/palette";
import { angularSep, project, type Viewport } from "../atlas/projection";

/** Outlines smaller than this (canvas px, long side) are drawn as markers instead. */
export const MIN_OUTLINE_PX = 8;
/** Pointer distance (px) from a marker that counts as hovering it. */
const HIT_RADIUS_PX = 9;
/** Angular radius assumed for clusters without an outline (unknown sensor size). */
const NO_HULL_RADIUS_DEG = 0.5;

/** Everything the renderer and hit-tester need to know about the current state. */
export interface CoverageScene {
  vp: Viewport;
  /** Clusters of the scopes currently shown. */
  clusters: app.CoverageCluster[];
  selected: ReadonlySet<string>;
  hoveredId: string | null;
  catalog: app.CatalogObject[];
  showLabels: boolean;
  palette: AtlasPalette;
  /** Target ring after flying to a catalog object. */
  focus: { ra: number; dec: number } | null;
  scopeColor(scope: string): RGB;
}

export type ClusterShape =
  | { kind: "poly"; cx: number; cy: number; pts: [number, number][]; span: number; area: number }
  | { kind: "dot"; cx: number; cy: number };

/** Screen geometry for a cluster: its outline when big enough, else a marker. */
export function clusterShape(vp: Viewport, c: app.CoverageCluster): ClusterShape | null {
  const centre = project(vp, c.ra, c.dec);
  if (!centre) return null;
  const [cx, cy] = centre;
  if (c.hull.length >= 3) {
    const pts: [number, number][] = [];
    for (const p of c.hull) {
      const q = project(vp, p.ra, p.dec);
      if (!q) return { kind: "dot", cx, cy };
      pts.push(q);
    }
    const span = polygonSpan(pts);
    if (span >= MIN_OUTLINE_PX) return { kind: "poly", cx, cy, pts, span, area: polygonArea(pts) };
  }
  return { kind: "dot", cx, cy };
}

/**
 * The cluster under (x, y): the nearest marker within HIT_RADIUS_PX wins;
 * otherwise the smallest outline containing the point, so a single framing
 * inside a wide mosaic stays clickable.
 */
export function hitTest(scene: CoverageScene, x: number, y: number): app.CoverageCluster | null {
  let bestDot: app.CoverageCluster | null = null;
  let bestDist = HIT_RADIUS_PX;
  let bestPoly: app.CoverageCluster | null = null;
  let bestArea = Infinity;
  for (const c of scene.clusters) {
    const s = clusterShape(scene.vp, c);
    if (!s) continue;
    if (s.kind === "dot") {
      const d = Math.hypot(x - s.cx, y - s.cy);
      if (d <= bestDist) {
        bestDist = d;
        bestDot = c;
      }
    } else if (s.area < bestArea && pointInPolygon(x, y, s.pts)) {
      bestArea = s.area;
      bestPoly = c;
    }
  }
  return bestDot ?? bestPoly;
}

/** Angular radius (degrees) of a cluster: centre to its farthest outline corner. */
export function clusterRadius(c: app.CoverageCluster): number {
  let r = 0;
  for (const p of c.hull) r = Math.max(r, angularSep(c.ra, c.dec, p.ra, p.dec));
  return r || NO_HULL_RADIUS_DEG;
}

/** Whether a cluster has a position on the sky (unplaced groups don't). */
export function isPlaced(c: app.CoverageCluster): boolean {
  return c.ra !== 0 || c.dec !== 0;
}
