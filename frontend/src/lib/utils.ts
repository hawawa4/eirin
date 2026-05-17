import type { app, fits } from "../../wailsjs/go/models";

export function isFits(name: string): boolean {
  const l = name.toLowerCase();
  return l.endsWith(".fits") || l.endsWith(".fit");
}

export function truncatePath(path: string, maxLen = 60): string {
  if (path.length <= maxLen) return path;
  const parts = path.split("/");
  if (parts.length <= 2) return "…" + path.slice(-(maxLen - 1));
  return "…/" + parts.slice(-2).join("/");
}

export function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatExpTime(secs: number): string {
  if (!secs) return "—";
  if (secs >= 60) return (secs / 60).toFixed(1) + " min";
  return secs + " s";
}

export function formatSize(bytes: number, isDir: boolean): string {
  if (isDir) return "—";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + " MB";
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + " GB";
}

export function getCellValue(entry: app.EnrichedFileEntry, colId: string): string {
  switch (colId) {
    case "name":
      return entry.name;
    case "object":
      return entry.hasMeta ? entry.object || "—" : entry.isDir ? "—" : "";
    case "filter":
      return entry.hasMeta ? entry.filter || "—" : entry.isDir ? "—" : "";
    case "dateObs":
      return entry.hasMeta
        ? entry.dateObs
          ? entry.dateObs.substring(0, 10)
          : "—"
        : entry.isDir
          ? "—"
          : "";
    case "expTime":
      return entry.hasMeta ? formatExpTime(entry.expTime) : entry.isDir ? "—" : "";
    case "modTime":
      return formatDate(entry.modTime);
    case "size":
      return formatSize(entry.size, entry.isDir);
    case "gain":
      return entry.hasMeta ? (entry.gain ? String(entry.gain) : "—") : "";
    case "ccdTemp":
      return entry.hasMeta ? (entry.ccdTemp ? entry.ccdTemp.toFixed(1) + " °C" : "—") : "";
    default:
      return "—";
  }
}

export function getLibraryCellValue(frame: app.LibraryFrame, colId: string): string {
  switch (colId) {
    case "name":
      return frame.fileName;
    case "object":
      return frame.object || "—";
    case "filter":
      return frame.filter || "—";
    case "dateObs":
      return frame.dateObs ? frame.dateObs.substring(0, 10) : "—";
    case "expTime":
      return formatExpTime(frame.expTime);
    case "size":
      return formatSize(frame.fileSize, false);
    case "gain":
      return frame.gain ? String(frame.gain) : "—";
    case "ccdTemp":
      return frame.ccdTemp ? frame.ccdTemp.toFixed(1) + " °C" : "—";
    case "telescope":
      return frame.telescope || "—";
    case "instrument":
      return frame.instrument || "—";
    case "fwhm":
      return frame.qualityAnalyzed && frame.fwhm
        ? `${frame.fwhm.toFixed(2)} ${frame.fwhmUnit || "px"}`
        : "—";
    case "starCount":
      return frame.qualityAnalyzed && frame.starCount ? String(frame.starCount) : "—";
    case "background":
      return frame.qualityAnalyzed && frame.background ? `${frame.background.toFixed(1)} ADU` : "—";
    case "noise":
      return frame.qualityAnalyzed && frame.noise ? `${frame.noise.toFixed(2)} ADU` : "—";
    case "snr":
      return frame.qualityAnalyzed && frame.snr ? frame.snr.toFixed(1) : "—";
    case "moonPhase": {
      if (frame.moonPhase < 0) return "—";
      const pct = Math.round(frame.moonPhase * 100);
      const icon = pct < 10 ? "🌑" : pct < 35 ? "🌒" : pct < 65 ? "🌓" : pct < 90 ? "🌔" : "🌕";
      return `${icon} ${pct}%`;
    }
    default:
      return "—";
  }
}

export function getFrameTextVal(f: app.LibraryFrame, colId: string): string {
  switch (colId) {
    case "name":
      return f.fileName;
    case "object":
      return f.object;
    case "filter":
      return f.filter;
    case "telescope":
      return f.telescope;
    case "instrument":
      return f.instrument;
    case "dateObs":
      return f.dateObs;
    default:
      return "";
  }
}

export function getFrameNumVal(f: app.LibraryFrame, colId: string): number | null {
  switch (colId) {
    case "expTime":
      return f.expTime;
    case "size":
      return f.fileSize;
    case "gain":
      return f.gain;
    case "ccdTemp":
      return f.ccdTemp;
    case "fwhm":
      return f.qualityAnalyzed ? f.fwhm : null;
    case "starCount":
      return f.qualityAnalyzed ? f.starCount : null;
    case "background":
      return f.qualityAnalyzed ? f.background : null;
    case "noise":
      return f.qualityAnalyzed ? f.noise : null;
    case "snr":
      return f.qualityAnalyzed ? f.snr : null;
    default:
      return null;
  }
}

export function getFrameSortVal(f: app.LibraryFrame, col: string): number | string {
  switch (col) {
    case "fwhm":
      return f.qualityAnalyzed ? f.fwhm : Infinity;
    case "starCount":
      return f.qualityAnalyzed ? -f.starCount : Infinity;
    case "background":
      return f.qualityAnalyzed ? f.background : Infinity;
    case "noise":
      return f.qualityAnalyzed ? f.noise : Infinity;
    case "snr":
      return f.qualityAnalyzed ? -f.snr : Infinity;
    case "expTime":
      return -f.expTime;
    case "gain":
      return f.gain;
    case "size":
      return -f.fileSize;
    default:
      return String((f as unknown as Record<string, unknown>)[col] ?? "");
  }
}

export function formatRA(degrees: number): string {
  const hours = degrees / 15;
  const h = Math.floor(hours);
  const m = Math.floor((hours - h) * 60);
  const s = ((hours - h) * 60 - m) * 60;
  return `${String(h).padStart(2, "0")}h ${String(m).padStart(2, "0")}m ${s.toFixed(1).padStart(4, "0")}s`;
}

export function formatDec(degrees: number): string {
  const sign = degrees < 0 ? "−" : "+";
  const abs = Math.abs(degrees);
  const d = Math.floor(abs);
  const m = Math.floor((abs - d) * 60);
  const s = ((abs - d) * 60 - m) * 60;
  return `${sign}${String(d).padStart(2, "0")}° ${String(m).padStart(2, "0")}' ${s.toFixed(1).padStart(4, "0")}"`;
}

export interface MetaRow {
  key: string;
  val: string;
}

export function basicRows(h: fits.FITSHeader | null): MetaRow[] {
  if (!h) return [];
  const expStr = !h.exptime
    ? "—"
    : h.exptime >= 60
      ? (h.exptime / 60).toFixed(1) + " min"
      : h.exptime + " s";
  return [
    { key: "Object", val: h.object || "—" },
    { key: "Filter", val: h.filter || "—" },
    { key: "Exposure", val: expStr },
    { key: "Date", val: h.dateObs || "—" },
    {
      key: "Size",
      val:
        h.width && h.height
          ? `${h.width} × ${h.height}${h.channels > 1 ? ` × ${h.channels}` : ""}`
          : "—",
    },
  ];
}

export function advancedRows(h: fits.FITSHeader | null): MetaRow[] {
  if (!h) return [];
  return [
    { key: "Gain", val: h.gain ? String(h.gain) : "—" },
    { key: "CCD Temp", val: h.ccdTemp ? h.ccdTemp + " °C" : "—" },
    { key: "Telescope", val: h.telescope || "—" },
    { key: "Camera", val: h.instrument || "—" },
    { key: "Binning", val: h.xbinning ? `${h.xbinning} × ${h.ybinning}` : "—" },
  ];
}
