// ── Fit-to-frames ───────────────────────────────────────────────────────────
// Computes a view (centre + zoom) that shows a set of sky targets with margin.
//
// Centre: the spherical mean (average of 3-D unit vectors) — the 2-D analogue of
// a circular mean for RA, so targets either side of RA 0h/24h average correctly.
//
// Spread-out libraries: a gnomonic view can't sensibly show targets more than
// ~60° from its centre (and a 120°+ wide view is unreadable anyway). When the
// targets span more than MAX_SPAN_DEG, we fit the densest cluster instead —
// the region (CLUSTER_RADIUS_DEG) containing the most frames. The atlas index
// carries no dates, so "most recently added" isn't available; frame count is
// the best proxy for "where the user's data is". Users can still reach other
// targets via the object list.

import { DEG, MAX_PPD, MIN_PPD, angularSep, fromTangent, normRA, toTangent } from "./projection";

export interface FitTarget {
  ra: number;
  dec: number;
  /** Half the angular extent of the target, degrees. */
  radius: number;
}

export interface FitResult {
  ra: number;
  dec: number;
  ppd: number;
}

const MAX_SPAN_DEG = 120;
const CLUSTER_RADIUS_DEG = 40;
/** Fraction of the canvas left free around the fitted targets (each side). */
const MARGIN = 0.12;
/** Never zoom out wider than this field for a fit (keeps the view legible). */
const MAX_FIT_FOV_DEG = 140;

export function sphericalMean(targets: { ra: number; dec: number }[]): [number, number] {
  let x = 0;
  let y = 0;
  let z = 0;
  for (const t of targets) {
    const c = Math.cos(t.dec * DEG);
    x += c * Math.cos(t.ra * DEG);
    y += c * Math.sin(t.ra * DEG);
    z += Math.sin(t.dec * DEG);
  }
  const n = Math.hypot(x, y, z);
  if (n < 1e-9) return [targets[0]?.ra ?? 0, targets[0]?.dec ?? 0];
  return [normRA(Math.atan2(y, x) / DEG), Math.asin(z / n) / DEG];
}

/** Collapses near-duplicate positions (many frames per target) into weighted buckets. */
function bucket(targets: FitTarget[]): (FitTarget & { weight: number })[] {
  const map = new Map<string, FitTarget & { weight: number }>();
  for (const t of targets) {
    const key = `${Math.round(t.ra * 2)}:${Math.round(t.dec * 2)}`;
    const b = map.get(key);
    if (b) {
      b.weight++;
      b.radius = Math.max(b.radius, t.radius);
    } else {
      map.set(key, { ...t, weight: 1 });
    }
  }
  return [...map.values()];
}

/** Returns the subset of targets forming the densest region, if the set is too spread out. */
export function densestCluster(targets: FitTarget[]): FitTarget[] {
  const [cra, cdec] = sphericalMean(targets);
  const span = Math.max(...targets.map((t) => angularSep(cra, cdec, t.ra, t.dec) + t.radius)) * 2;
  if (span <= MAX_SPAN_DEG) return targets;

  const buckets = bucket(targets);
  let best = buckets[0];
  let bestWeight = -1;
  for (const b of buckets) {
    let w = 0;
    for (const o of buckets)
      if (angularSep(b.ra, b.dec, o.ra, o.dec) <= CLUSTER_RADIUS_DEG) w += o.weight;
    if (w > bestWeight) {
      bestWeight = w;
      best = b;
    }
  }
  return targets.filter((t) => angularSep(best.ra, best.dec, t.ra, t.dec) <= CLUSTER_RADIUS_DEG);
}

/**
 * Fits `targets` into a w×h canvas. With `cluster` (default) a widely spread set
 * is reduced to its densest cluster first. `maxPpd` caps zoom for tiny targets.
 */
export function fitView(
  targets: FitTarget[],
  w: number,
  h: number,
  opts: { cluster?: boolean; maxPpd?: number } = {},
): FitResult | null {
  if (targets.length === 0 || w <= 0 || h <= 0) return null;
  const set = opts.cluster === false ? targets : densestCluster(targets);

  // Two passes: centre on the mean, measure the tangent-plane bounding box,
  // then re-centre on the bbox middle so asymmetric sets are framed evenly.
  let [ra0, dec0] = sphericalMean(set);
  let bbox = tangentBBox(set, ra0, dec0);
  if (bbox) {
    [ra0, dec0] = fromTangent(ra0, dec0, (bbox[0] + bbox[1]) / 2, (bbox[2] + bbox[3]) / 2);
    bbox = tangentBBox(set, ra0, dec0) ?? bbox;
  }
  const spanX = bbox ? Math.max(bbox[1] - bbox[0], 0.05) : MAX_FIT_FOV_DEG;
  const spanY = bbox ? Math.max(bbox[3] - bbox[2], 0.05) : MAX_FIT_FOV_DEG;

  const usableW = w * (1 - 2 * MARGIN);
  const usableH = h * (1 - 2 * MARGIN);
  let ppd = Math.min(usableW / spanX, usableH / spanY);
  ppd = Math.max(ppd, w / MAX_FIT_FOV_DEG, MIN_PPD);
  ppd = Math.min(ppd, opts.maxPpd ?? MAX_PPD);
  return { ra: ra0, dec: Math.max(-89.9, Math.min(89.9, dec0)), ppd };
}

/** [minXi, maxXi, minEta, maxEta] in tangent degrees, or null if any target is behind the plane. */
function tangentBBox(
  set: FitTarget[],
  ra0: number,
  dec0: number,
): [number, number, number, number] | null {
  let minX = Infinity;
  let maxX = -Infinity;
  let minY = Infinity;
  let maxY = -Infinity;
  for (const t of set) {
    const p = toTangent(ra0, dec0, t.ra, t.dec);
    if (!p) return null;
    minX = Math.min(minX, p[0] - t.radius);
    maxX = Math.max(maxX, p[0] + t.radius);
    minY = Math.min(minY, p[1] - t.radius);
    maxY = Math.max(maxY, p[1] + t.radius);
  }
  return [minX, maxX, minY, maxY];
}
