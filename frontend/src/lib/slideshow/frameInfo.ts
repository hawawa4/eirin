// ── Basic metadata for the slideshow's info card ────────────────────────────
// Processed images are usually exports (PNG/JPEG/TIFF) with no FITS header, so
// most of what the card shows comes from the file name — Seestar names look
// like "Stacked_30_M 42_10.0s_IRCUT_20240101-221530.jpg". Whatever the name
// doesn't say falls back to what the indexer read from the file.

import type * as app from "$models/app";

export interface FrameInfo {
  title: string;
  /** Label/value pairs, only for what could be worked out. */
  rows: [label: string, value: string][];
}

interface NameMeta {
  telescope?: string;
  /** Number of stacked subs, when the name says so. */
  subs?: number;
  /** Exposure of one sub, seconds. */
  subExposure?: number;
  filter?: string;
  /** Local calendar date as [year, month, day]. */
  date?: [number, number, number];
}

const FILTER_LABELS: Record<string, string> = { ircut: "IR-cut", "ir-cut": "IR-cut" };

/** Reads whatever the file name reveals; everything is optional. */
export function parseFrameName(fileName: string): NameMeta {
  const stem = fileName.replace(/\.[A-Za-z0-9]{2,5}$/, "");
  const meta: NameMeta = {};

  const scope = stem.match(/(?<![A-Za-z0-9])(S50|S30)(?:[\s_-]?(Pro))?(?![A-Za-z0-9])/i);
  if (scope) meta.telescope = `${scope[1].toUpperCase()}${scope[2] ? " Pro" : ""}`;

  // "30x10s" in one go, else "Stacked_30_" for the count and "10.0s" for the exposure.
  const combined = stem.match(/(?<![A-Za-z0-9])(\d+)\s?[x×]\s?(\d+(?:\.\d+)?)\s?s(?![A-Za-z])/i);
  if (combined) {
    meta.subs = Number(combined[1]);
    meta.subExposure = Number(combined[2]);
  } else {
    const count = stem.match(/^(?:stacked|stack|sum)[_\s-](\d+)(?![\d.])/i);
    if (count) meta.subs = Number(count[1]);
    const exp = stem.match(/(?<![A-Za-z0-9.])(\d+(?:\.\d+)?)\s?s(?![A-Za-z])/);
    if (exp) meta.subExposure = Number(exp[1]);
  }

  const filter = stem.match(/(?<![A-Za-z0-9])(IR-?CUT|LP|DUO-?BAND)(?![A-Za-z0-9])/i);
  if (filter) {
    const key = filter[1].toLowerCase();
    meta.filter = FILTER_LABELS[key] ?? filter[1].toUpperCase();
  }

  const date = stem.match(
    /(?<!\d)((?:19|20)\d{2})[-_]?(0[1-9]|1[0-2])[-_]?(0[1-9]|[12]\d|3[01])(?!\d)/,
  );
  if (date) meta.date = [Number(date[1]), Number(date[2]), Number(date[3])];

  return meta;
}

/** "S50 Pro_8dc7900b" → "S50 Pro": Seestar firmware appends the serial after an underscore. */
export function scopeLabel(telescope: string): string {
  const m = telescope.match(/^(.+)_[0-9a-f]{4,}$/i);
  return m ? m[1] : telescope;
}

export function formatDuration(totalSeconds: number): string {
  const s = Math.round(totalSeconds);
  if (s < 60) return `${s} s`;
  if (s < 3600) {
    const m = Math.floor(s / 60);
    return s % 60 === 0 ? `${m} min` : `${m} min ${s % 60} s`;
  }
  const h = Math.floor(s / 3600);
  const m = Math.round((s % 3600) / 60);
  return m === 0 ? `${h} h` : `${h} h ${m} min`;
}

function formatDate([y, m, d]: [number, number, number]): string {
  // Built from local components so the day never shifts with the time zone.
  return new Date(y, m - 1, d).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

/** The DATE-OBS calendar day ("2024-01-01T22:15:30" → [2024, 1, 1]). */
function dateFromHeader(dateObs: string): [number, number, number] | undefined {
  const m = dateObs.match(/^(\d{4})-(\d{2})-(\d{2})/);
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : undefined;
}

function stripExtension(fileName: string): string {
  return fileName.replace(/\.[A-Za-z0-9]{2,5}$/, "");
}

export function describeFrame(f: app.LibraryFrame): FrameInfo {
  const name = parseFrameName(f.fileName);
  const rows: FrameInfo["rows"] = [];

  const telescope = name.telescope ?? (f.telescope ? scopeLabel(f.telescope) : undefined);
  if (telescope) rows.push(["Telescope", telescope]);

  if (name.subs && name.subExposure) {
    const total = formatDuration(name.subs * name.subExposure);
    rows.push(["Exposure", `${name.subs} × ${formatDuration(name.subExposure)} = ${total}`]);
  } else if (name.subExposure) {
    rows.push(["Exposure", formatDuration(name.subExposure)]);
  } else if (f.expTime > 0) {
    rows.push(["Exposure", formatDuration(f.expTime)]);
  }

  const date = name.date ?? dateFromHeader(f.dateObs);
  if (date) rows.push(["Date", formatDate(date)]);

  const filter = name.filter ?? (f.filter || undefined);
  if (filter) rows.push(["Filter", filter]);

  return { title: f.object || stripExtension(f.fileName), rows };
}
