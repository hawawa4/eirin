// ── Object list & catalog search ────────────────────────────────────────────

import type * as app from "$models/app";
import { sphericalMean } from "./fit";

export interface ObjectGroup {
  name: string;
  ra: number;
  dec: number;
  frames: app.AtlasIndexEntry[];
}

/** The key frames are grouped under in the object list. */
export function objectKey(e: app.AtlasIndexEntry): string {
  return e.object || e.name;
}

export function isFinalImage(e: app.AtlasIndexEntry): boolean {
  return e.frameType === "processed" || e.frameType === "image";
}

/** Lower-cases and strips spaces/punctuation so "m 42", "M42" and "m-42" all match. */
export function normalizeQuery(s: string): string {
  return s.toLowerCase().replace(/[\s_\-.]+/g, "");
}

export function buildObjectGroups(entries: app.AtlasIndexEntry[]): ObjectGroup[] {
  const map = new Map<string, app.AtlasIndexEntry[]>();
  for (const e of entries) {
    const key = objectKey(e);
    const list = map.get(key);
    if (list) list.push(e);
    else map.set(key, [e]);
  }
  const groups: ObjectGroup[] = [];
  for (const [name, frames] of map) {
    const [ra, dec] = sphericalMean(frames);
    groups.push({ name, ra, dec, frames });
  }
  return groups.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }));
}

export function filterGroups(groups: ObjectGroup[], query: string): ObjectGroup[] {
  const q = query.trim().toLowerCase();
  if (!q) return groups;
  const nq = normalizeQuery(q);
  return groups.filter(
    (g) => g.name.toLowerCase().includes(q) || (nq && normalizeQuery(g.name).includes(nq)),
  );
}

/**
 * The frame to select when an object is picked: final images first, then
 * stacked frames; among equals the last by name (file names usually embed
 * the capture date, so this is typically the most recent).
 */
export function topFrame(frames: app.AtlasIndexEntry[]): app.AtlasIndexEntry | null {
  let best: app.AtlasIndexEntry | null = null;
  const rank = (e: app.AtlasIndexEntry) =>
    e.frameType === "processed"
      ? 3
      : e.frameType === "image"
        ? 2
        : e.frameType === "stacked"
          ? 1
          : 0;
  for (const f of frames) {
    if (!best || rank(f) > rank(best) || (rank(f) === rank(best) && f.name > best.name)) best = f;
  }
  return best;
}

const MAX_CATALOG_RESULTS = 25;

/** Client-side catalog search (matches "M42", "m 42", "orion"…); prefix matches and bright objects first. */
export function searchCatalog(catalog: app.CatalogObject[], query: string): app.CatalogObject[] {
  const q = query.trim().toLowerCase();
  if (!q) return [];
  const nq = normalizeQuery(q);
  const scored: { obj: app.CatalogObject; score: number }[] = [];
  for (const obj of catalog) {
    const name = obj.name.toLowerCase();
    const nname = normalizeQuery(name);
    let score = -1;
    if (nname === nq) score = 0;
    else if (name.startsWith(q) || nname.startsWith(nq)) score = 1;
    else if (name.split(/\s+/).some((w) => w.startsWith(q))) score = 2;
    else if (name.includes(q) || (nq && nname.includes(nq))) score = 3;
    if (score >= 0) scored.push({ obj, score });
  }
  scored.sort((a, b) => a.score - b.score || a.obj.mag - b.obj.mag);
  return scored.slice(0, MAX_CATALOG_RESULTS).map((s) => s.obj);
}
