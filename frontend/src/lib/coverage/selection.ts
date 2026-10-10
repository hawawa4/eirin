// ── Coverage selection helpers: names, totals, neighbours, object list ──────

import type * as app from "$models/app";
import type { NamedGroup } from "../atlas/objects";
import { angularSep } from "../atlas/projection";
import { clusterRadius, isPlaced } from "./scene";

/** How much two outlines' radii may overlap, as a fraction, to count as neighbours. */
const NEIGHBOUR_OVERLAP = 0.8;
/** Max neighbours offered in the selection panel. */
const MAX_NEIGHBOURS = 8;

/** "M 42", "M 42 + NGC 1977", "M 42 + NGC 1977 +2". */
export function clusterName(c: app.CoverageCluster): string {
  const names = c.objects.map((o) => o.name || "Unknown");
  if (names.length <= 2) return names.join(" + ");
  return `${names.slice(0, 2).join(" + ")} +${names.length - 2}`;
}

/** Total integration: "2h 05m", "45m", "30s". */
export function formatIntegration(seconds: number): string {
  const s = Math.round(seconds);
  if (s < 60) return `${s}s`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m`;
  return `${Math.floor(m / 60)}h ${String(m % 60).padStart(2, "0")}m`;
}

/** "Mar 12, 2026" or "Mar 12 – Apr 3, 2026" from DATE-OBS values. */
export function formatDateRange(first: string, last: string): string {
  if (!first) return "";
  const fmt = (d: string, year: boolean) =>
    new Date(d).toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      ...(year ? { year: "numeric" } : {}),
    });
  const a = first.slice(0, 10);
  const b = (last || first).slice(0, 10);
  if (a === b) return fmt(first, true);
  const sameYear = a.slice(0, 4) === b.slice(0, 4);
  return `${fmt(first, !sameYear)} – ${fmt(last, true)}`;
}

export interface SelectionSummary {
  frames: number;
  expTotal: number;
  /** Object tags across the selection, most subs first. */
  objects: app.CoverageObject[];
  scopes: string[];
  filters: string[];
  firstDate: string;
  lastDate: string;
  paths: string[];
}

export function summarize(clusters: app.CoverageCluster[]): SelectionSummary {
  const objects = new Map<string, number>();
  const scopes = new Set<string>();
  const filters = new Set<string>();
  let frames = 0;
  let expTotal = 0;
  let firstDate = "";
  let lastDate = "";
  const paths: string[] = [];
  for (const c of clusters) {
    frames += c.frames;
    expTotal += c.expTotal;
    scopes.add(c.scope);
    for (const f of c.filters) filters.add(f);
    for (const o of c.objects) objects.set(o.name, (objects.get(o.name) ?? 0) + o.count);
    if (c.firstDate && (!firstDate || c.firstDate < firstDate)) firstDate = c.firstDate;
    if (c.lastDate > lastDate) lastDate = c.lastDate;
    paths.push(...c.paths);
  }
  return {
    frames,
    expTotal,
    objects: [...objects]
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name)),
    scopes: [...scopes],
    filters: [...filters].sort(),
    firstDate,
    lastDate,
    paths,
  };
}

/**
 * Unselected clusters whose outlines overlap the selection's, nearest first:
 * the other framings, tags and scopes that could go into the same project.
 */
export function neighbours(
  all: app.CoverageCluster[],
  selected: app.CoverageCluster[],
): app.CoverageCluster[] {
  const ids = new Set(selected.map((c) => c.id));
  const placed = selected.filter(isPlaced).map((c) => ({ c, r: clusterRadius(c) }));
  if (placed.length === 0) return [];
  const found: { c: app.CoverageCluster; d: number }[] = [];
  for (const c of all) {
    if (ids.has(c.id) || !isPlaced(c)) continue;
    const r = clusterRadius(c);
    let best = Infinity;
    for (const s of placed) {
      const d = angularSep(c.ra, c.dec, s.c.ra, s.c.dec);
      if (d < (r + s.r) * NEIGHBOUR_OVERLAP) best = Math.min(best, d);
    }
    if (best < Infinity) found.push({ c, d: best });
  }
  return found
    .sort((a, b) => a.d - b.d)
    .slice(0, MAX_NEIGHBOURS)
    .map((f) => f.c);
}

/** A default project name: the selection's main object, plus the scope when there's one. */
export function suggestName(sum: SelectionSummary, scopeLabel: (scope: string) => string): string {
  const object = sum.objects[0]?.name ?? "";
  if (!object) return "";
  return sum.scopes.length === 1 ? `${object} ${scopeLabel(sum.scopes[0])}` : object;
}

/** An OBJECT tag in the object list, with the clusters that contain it. */
export interface CoverageGroup extends NamedGroup {
  clusterIds: string[];
}

/** Object list entries: each tag with its sub count and clusters, by name. */
export function objectGroups(clusters: app.CoverageCluster[]): CoverageGroup[] {
  const map = new Map<string, CoverageGroup>();
  for (const c of clusters) {
    for (const o of c.objects) {
      const name = o.name || "Unknown";
      let g = map.get(name);
      if (!g) {
        g = { name, count: 0, clusterIds: [] };
        map.set(name, g);
      }
      g.count += o.count;
      g.clusterIds.push(c.id);
    }
  }
  return [...map.values()].sort((a, b) =>
    a.name.localeCompare(b.name, undefined, { numeric: true }),
  );
}
