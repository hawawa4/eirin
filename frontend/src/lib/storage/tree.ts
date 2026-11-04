// ── Storage tree ────────────────────────────────────────────────────────────
// Disk usage by object → night, built from the library frames so every node
// knows its files and can be acted on (reject, delete, show in folder…).

import type * as app from "$models/app";

export const UNKNOWN_OBJECT = "(unknown object)";
export const NO_DATE = "(no date)";

export interface StorageGroup {
  /** Unique within its level: the object name, or the YYYY-MM-DD night. */
  label: string;
  /** Raw object ("" when unknown); set on every node. */
  object: string;
  /** YYYY-MM-DD ("" when unknown); only on night-level nodes. */
  date?: string;
  totalBytes: number;
  frameCount: number;
  paths: string[];
  rejectedBytes: number;
  rejectedPaths: string[];
  children?: StorageGroup[];
}

export interface TypeTotal {
  type: string;
  bytes: number;
  count: number;
}

export interface StorageSummary {
  root: StorageGroup;
  byType: TypeTotal[];
  rejectedBytes: number;
  rejectedCount: number;
}

const TYPE_ORDER = ["light", "dark", "flat", "bias", "stacked", "processed", "image"];

function emptyGroup(label: string, object: string, date?: string): StorageGroup {
  return {
    label,
    object,
    date,
    totalBytes: 0,
    frameCount: 0,
    paths: [],
    rejectedBytes: 0,
    rejectedPaths: [],
  };
}

function add(g: StorageGroup, f: app.LibraryFrame) {
  g.totalBytes += f.fileSize;
  g.frameCount++;
  g.paths.push(f.nasPath);
  if (f.isRejected) {
    g.rejectedBytes += f.fileSize;
    g.rejectedPaths.push(f.nasPath);
  }
}

const bySizeDesc = (a: StorageGroup, b: StorageGroup) => b.totalBytes - a.totalBytes;

/**
 * Builds the object → night tree of `frames`, keeping only frames whose type is
 * in `types` (all frames when empty) and, with `rejectedOnly`, only rejected ones.
 * The per-type totals always cover every frame, so the type chips keep their numbers.
 */
export function buildStorage(
  frames: readonly app.LibraryFrame[],
  types: ReadonlySet<string>,
  rejectedOnly: boolean,
): StorageSummary {
  const root = emptyGroup("All objects", "");
  const objects = new Map<string, StorageGroup>();
  const nights = new Map<StorageGroup, Map<string, StorageGroup>>();
  const typeTotals = new Map<string, TypeTotal>();
  let rejectedBytes = 0;
  let rejectedCount = 0;

  for (const f of frames) {
    const t = typeTotals.get(f.frameType) ?? { type: f.frameType, bytes: 0, count: 0 };
    t.bytes += f.fileSize;
    t.count++;
    typeTotals.set(f.frameType, t);
    if (f.isRejected) {
      rejectedBytes += f.fileSize;
      rejectedCount++;
    }

    if (types.size > 0 && !types.has(f.frameType)) continue;
    if (rejectedOnly && !f.isRejected) continue;

    const objLabel = f.object || UNKNOWN_OBJECT;
    let obj = objects.get(objLabel);
    if (!obj) {
      obj = emptyGroup(objLabel, f.object);
      objects.set(objLabel, obj);
      nights.set(obj, new Map());
    }
    const date = f.dateObs.length >= 10 ? f.dateObs.slice(0, 10) : "";
    const nightLabel = date || NO_DATE;
    const objNights = nights.get(obj)!;
    let night = objNights.get(nightLabel);
    if (!night) {
      night = emptyGroup(nightLabel, f.object, date);
      objNights.set(nightLabel, night);
    }
    add(root, f);
    add(obj, f);
    add(night, f);
  }

  root.children = [...objects.values()].sort(bySizeDesc);
  for (const obj of root.children) {
    obj.children = [...nights.get(obj)!.values()].sort(bySizeDesc);
  }

  const rank = (t: string) => {
    const i = TYPE_ORDER.indexOf(t);
    return i === -1 ? 99 : i;
  };
  const byType = [...typeTotals.values()].sort((a, b) => rank(a.type) - rank(b.type));
  return { root, byType, rejectedBytes, rejectedCount };
}

/** The node at `path` (labels from the root's children down), or null if gone. */
export function nodeAt(root: StorageGroup, path: readonly string[]): StorageGroup | null {
  let node: StorageGroup | undefined = root;
  for (const label of path) {
    node = node?.children?.find((c) => c.label === label);
    if (!node) return null;
  }
  return node ?? null;
}

/** Longest prefix of `path` that still leads to a node with children. */
export function validPrefix(root: StorageGroup, path: readonly string[]): string[] {
  const out: string[] = [];
  let node: StorageGroup | undefined = root;
  for (const label of path) {
    node = node?.children?.find((c) => c.label === label);
    if (!node?.children?.length) break;
    out.push(label);
  }
  return out;
}

/** Deepest folder containing every path (the paths' shared parent directory). */
export function commonFolder(paths: readonly string[]): string {
  if (paths.length === 0) return "";
  const split = (p: string) => p.split(/[\\/]/).slice(0, -1);
  let common = split(paths[0]);
  for (const p of paths.slice(1)) {
    const parts = split(p);
    let i = 0;
    while (i < common.length && i < parts.length && common[i] === parts[i]) i++;
    common = common.slice(0, i);
    if (common.length === 0) break;
  }
  const sep = paths[0].includes("\\") && !paths[0].includes("/") ? "\\" : "/";
  return common.join(sep) || sep;
}
