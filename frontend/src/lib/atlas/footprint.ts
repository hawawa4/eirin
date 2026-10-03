// ── Frame footprints ────────────────────────────────────────────────────────
// Maps image pixels to the sky using the frame's centre, pixel scale and
// rotation, so the outline and the preview image share ONE transform.
//
// Rotation convention: `rotation` is CROTA2 (the angle from North to the image
// +Y axis, measured N→W; see internal/fits extractWCS). With the standard
// CDELT1 < 0 parity, the CD matrix gives, for an image offset (u right, v down,
// in pixels, preview rows already flipped to top-down):
//   xi  = s·(−cosR·u + sinR·v)     (East positive)
//   eta = s·(−sinR·u − cosR·v)     (North positive)
// On our East-left / North-up canvas that is exactly ctx.rotate(+R).

import type * as app from "$models/app";
import { DEG, fromTangent, project, type Viewport } from "./projection";

export interface FrameSize {
  width: number;
  height: number;
}

/** Fallback angular extent (degrees) for frames whose pixel size isn't known yet. */
export const UNKNOWN_EXTENT_DEG = 1.2;
/** Assumed long-side pixel count when estimating on-screen size before the real size is fetched. */
const ESTIMATE_PIXELS = 1600;

/** Sky position of image pixel offset (u, v) from the frame centre (pixels, v down). */
export function pixelToSky(
  entry: app.AtlasIndexEntry,
  rotationDeg: number,
  u: number,
  v: number,
): [number, number] {
  const s = entry.pixelScale / 3600;
  const r = rotationDeg * DEG;
  const xi = s * (-Math.cos(r) * u + Math.sin(r) * v);
  const eta = s * (-Math.sin(r) * u - Math.cos(r) * v);
  return fromTangent(entry.ra, entry.dec, xi, eta);
}

/**
 * Canvas positions of the image corners in order TL, TR, BR, BL (image space).
 * Returns null if any corner falls behind the projection plane.
 */
export function footprintCorners(
  vp: Viewport,
  entry: app.AtlasIndexEntry,
  sz: FrameSize,
  rotationDeg: number,
): [number, number][] | null {
  const hw = sz.width / 2;
  const hh = sz.height / 2;
  const out: [number, number][] = [];
  for (const [u, v] of [
    [-hw, -hh],
    [hw, -hh],
    [hw, hh],
    [-hw, hh],
  ]) {
    const [ra, dec] = pixelToSky(entry, rotationDeg, u, v);
    const p = project(vp, ra, dec);
    if (!p) return null;
    out.push(p);
  }
  return out;
}

/** Angular size (degrees) of the frame's long side, using the real size when known. */
export function frameExtentDeg(entry: app.AtlasIndexEntry, sz: FrameSize | undefined): number {
  if (sz) return (Math.hypot(sz.width, sz.height) * entry.pixelScale) / 3600;
  if (entry.pixelScale > 0) return (ESTIMATE_PIXELS * entry.pixelScale) / 3600;
  return UNKNOWN_EXTENT_DEG;
}

/** Longest side, in canvas pixels, of a projected footprint. */
export function polygonSpan(pts: [number, number][]): number {
  let minX = Infinity;
  let maxX = -Infinity;
  let minY = Infinity;
  let maxY = -Infinity;
  for (const [x, y] of pts) {
    if (x < minX) minX = x;
    if (x > maxX) maxX = x;
    if (y < minY) minY = y;
    if (y > maxY) maxY = y;
  }
  return Math.max(maxX - minX, maxY - minY);
}

export function polygonArea(pts: [number, number][]): number {
  let a = 0;
  for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
    a += (pts[j][0] + pts[i][0]) * (pts[j][1] - pts[i][1]);
  }
  return Math.abs(a / 2);
}

export function pointInPolygon(px: number, py: number, pts: [number, number][]): boolean {
  let inside = false;
  for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
    const [xi, yi] = pts[i];
    const [xj, yj] = pts[j];
    if (yi > py !== yj > py && px < ((xj - xi) * (py - yi)) / (yj - yi) + xi) inside = !inside;
  }
  return inside;
}

export function polygonOnScreen(pts: [number, number][], vp: Viewport, margin = 80): boolean {
  let minX = Infinity;
  let maxX = -Infinity;
  let minY = Infinity;
  let maxY = -Infinity;
  for (const [x, y] of pts) {
    if (x < minX) minX = x;
    if (x > maxX) maxX = x;
    if (y < minY) minY = y;
    if (y > maxY) maxY = y;
  }
  return maxX >= -margin && minX <= vp.w + margin && maxY >= -margin && minY <= vp.h + margin;
}
