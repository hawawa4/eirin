import type { browser, fits } from "../../wailsjs/go/models";

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

export function getCellValue(entry: browser.EnrichedFileEntry, colId: string): string {
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
