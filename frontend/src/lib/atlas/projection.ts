// ── Sky projection math ─────────────────────────────────────────────────────
// Gnomonic (tangent-plane) projection centred on the view, rendered with North
// up and East to the LEFT (the conventional "looking up at the sky" orientation,
// matching how FITS images display once their bottom-up rows are flipped).

export const DEG = Math.PI / 180;

/** Current camera: centre (RA/Dec, degrees), zoom (pixels per degree), canvas size. */
export interface Viewport {
  ra: number;
  dec: number;
  ppd: number;
  w: number;
  h: number;
}

export const MIN_PPD = 0.3;
export const MAX_PPD = 8000;

export function clampPpd(ppd: number): number {
  return Math.max(MIN_PPD, Math.min(MAX_PPD, ppd));
}

export function normRA(ra: number): number {
  return ((ra % 360) + 360) % 360;
}

/** Tangent-plane coordinates (degrees) of (ra, dec) about (ra0, dec0); null if behind the plane. */
export function toTangent(
  ra0: number,
  dec0: number,
  ra: number,
  dec: number,
): [number, number] | null {
  const d0 = dec0 * DEG;
  const d = dec * DEG;
  const dRA = (ra - ra0) * DEG;
  const denom = Math.sin(d0) * Math.sin(d) + Math.cos(d0) * Math.cos(d) * Math.cos(dRA);
  if (denom <= 0.001) return null;
  const xi = (Math.cos(d) * Math.sin(dRA)) / denom;
  const eta = (Math.cos(d0) * Math.sin(d) - Math.sin(d0) * Math.cos(d) * Math.cos(dRA)) / denom;
  return [xi / DEG, eta / DEG];
}

/** Inverse of toTangent: tangent-plane degrees about (ra0, dec0) → [ra, dec]. */
export function fromTangent(
  ra0: number,
  dec0: number,
  xiDeg: number,
  etaDeg: number,
): [number, number] {
  const xi = xiDeg * DEG;
  const eta = etaDeg * DEG;
  const rho = Math.hypot(xi, eta);
  if (rho < 1e-12) return [normRA(ra0), dec0];
  const d0 = dec0 * DEG;
  const c = Math.atan(rho);
  const dec = Math.asin(Math.cos(c) * Math.sin(d0) + (eta * Math.sin(c) * Math.cos(d0)) / rho);
  const ra =
    ra0 * DEG +
    Math.atan2(
      xi * Math.sin(c),
      rho * Math.cos(d0) * Math.cos(c) - eta * Math.sin(d0) * Math.sin(c),
    );
  return [normRA(ra / DEG), dec / DEG];
}

/** Sky → canvas pixels, or null when the point is on the far side of the sky. */
export function project(vp: Viewport, ra: number, dec: number): [number, number] | null {
  const t = toTangent(vp.ra, vp.dec, ra, dec);
  if (!t) return null;
  return [vp.w / 2 - t[0] * vp.ppd, vp.h / 2 - t[1] * vp.ppd];
}

/** Canvas pixels → sky [ra, dec]. */
export function unproject(vp: Viewport, x: number, y: number): [number, number] {
  return fromTangent(vp.ra, vp.dec, -(x - vp.w / 2) / vp.ppd, -(y - vp.h / 2) / vp.ppd);
}

/** Great-circle distance in degrees. */
export function angularSep(ra1: number, dec1: number, ra2: number, dec2: number): number {
  const d1 = dec1 * DEG;
  const d2 = dec2 * DEG;
  const dRA = (ra2 - ra1) * DEG;
  const a = Math.sin((d2 - d1) / 2) ** 2 + Math.cos(d1) * Math.cos(d2) * Math.sin(dRA / 2) ** 2;
  return (2 * Math.asin(Math.min(1, Math.sqrt(a)))) / DEG;
}

/**
 * Re-centres the view so the sky point under canvas pixel (x, y) stays put after
 * a zoom change from `oldPpd` → `vp.ppd`. Returns the new centre.
 */
export function zoomAnchored(
  vp: Viewport,
  oldPpd: number,
  x: number,
  y: number,
): { ra: number; dec: number } {
  const [aRA, aDec] = unproject({ ...vp, ppd: oldPpd }, x, y);
  // Iteratively nudge the centre until the anchor lands back under (x, y).
  // Moving the centre by (dξ, dη) shifts the anchor on screen by ≈ (dξ, dη)·ppd.
  let ra = vp.ra;
  let dec = vp.dec;
  for (let i = 0; i < 5; i++) {
    const p = project({ ...vp, ra, dec }, aRA, aDec);
    if (!p) break;
    const rx = x - p[0];
    const ry = y - p[1];
    if (Math.hypot(rx, ry) < 0.05) break;
    [ra, dec] = fromTangent(ra, dec, rx / vp.ppd, ry / vp.ppd);
  }
  return { ra, dec: Math.max(-89.9, Math.min(89.9, dec)) };
}

/**
 * Pans the view by a screen-pixel delta from a starting centre (drag-to-pan).
 * Dragging right moves the sky right, i.e. the centre moves East (RA increases).
 */
export function panFrom(
  startRA: number,
  startDec: number,
  ppd: number,
  dx: number,
  dy: number,
): { ra: number; dec: number } {
  const [ra, dec] = fromTangent(startRA, startDec, dx / ppd, dy / ppd);
  return { ra, dec: Math.max(-89.9, Math.min(89.9, dec)) };
}
