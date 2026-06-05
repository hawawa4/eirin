import type * as app from "$models/app";

export interface ColumnDef {
  id: string;
  label: string;
  visible: boolean;
  width: number;
  order: number;
}

export interface FileGroup {
  key: string;
  object: string;
  date: string;
  files: app.EnrichedFileEntry[];
}

export interface IndexProgress {
  phase: "scanning" | "indexing" | "done" | "cancelled";
  total: number;
  done: number;
  indexed: number;
  errors: number;
  current: string;
}

// Minimal interface both EnrichedFileEntry and LibraryFrame satisfy (via mapping).
export interface CtxEntry {
  path: string;
  name: string;
  isRejected: boolean;
  frameType: string;
}

export interface CtxMenuState {
  x: number;
  y: number;
  entry: CtxEntry;
  sirilAvailable: boolean;
  selectionCount: number; // 1 = single, >1 = multi-select batch
}

export interface SirilInfo {
  executable: string;
  version: string;
  available: boolean;
}

export interface AnalysisProgress {
  phase: "analyzing" | "done" | "cancelled";
  total: number;
  done: number;
  current: string;
  errors: number;
}

export type ViewMode = "files" | "rejected";

export type AppMode =
  | "browser"
  | "library"
  | "atlas"
  | "import"
  | "projects"
  | "storage"
  | "settings";

export type Theme = "blue" | "red" | "grey";

export type FrameType = "light" | "dark" | "flat" | "bias" | "stacked" | "processed";

export interface ColFilter {
  text?: string;
  numOp?: "<" | ">";
  numVal?: number | null;
  types?: FrameType[];
}

export type LibraryGroupBy = "object" | "date" | "filter" | "frameType";

export type ImportState = "idle" | "scanning" | "scanned" | "importing" | "done" | "error";

export interface AppInfo {
  dbPath: string;
  serverPort: number;
  serverUrl: string;
  portSource: string;
}

export interface ImportCandidate {
  sourcePath: string;
  relativePath: string;
  destPath: string;
  fileSize: number;
}

export interface ImportProgress {
  phase: "copying" | "done" | "error";
  current: number;
  total: number;
  currentFile: string;
  copied: number;
  skipped: number;
  error?: string;
}

export interface Project {
  id: number;
  name: string;
  description: string;
  folder: string;
  createdAt: string;
}

export interface ProjectOutputFile {
  name: string;
  path: string;
  size: number;
  modTime: string;
}

export interface LibraryGroup {
  key: string;
  label: string;
  frames: app.LibraryFrame[];
}

export const FRAME_TYPE_META: Record<
  string,
  { label: string; short: string; color: string; bg: string }
> = {
  light: { label: "Light", short: "LIGHT", color: "#60a5fa", bg: "#1e3a5f" },
  dark: { label: "Dark", short: "DARK", color: "#94a3b8", bg: "#1e2a3a" },
  flat: { label: "Flat", short: "FLAT", color: "#fbbf24", bg: "#3d2a00" },
  bias: { label: "Bias", short: "BIAS", color: "#a78bfa", bg: "#2d1f4a" },
  stacked: { label: "Stacked", short: "STACK", color: "#34d399", bg: "#0d3a2a" },
  processed: { label: "Processed", short: "PROC", color: "#f59e0b", bg: "#3d2d00" },
};

export const DEFAULT_LIBRARY_COLUMNS: ColumnDef[] = [
  { id: "frameType", label: "Type", visible: true, width: 70, order: 0 },
  { id: "name", label: "Name", visible: true, width: 210, order: 1 },
  { id: "object", label: "Object", visible: true, width: 120, order: 2 },
  { id: "filter", label: "Filter", visible: true, width: 70, order: 3 },
  { id: "dateObs", label: "Date", visible: true, width: 130, order: 4 },
  { id: "expTime", label: "Exp", visible: true, width: 65, order: 5 },
  { id: "size", label: "Size", visible: false, width: 75, order: 6 },
  { id: "gain", label: "Gain", visible: false, width: 60, order: 7 },
  { id: "ccdTemp", label: "Temp", visible: false, width: 75, order: 8 },
  { id: "telescope", label: "Telescope", visible: false, width: 120, order: 9 },
  { id: "instrument", label: "Camera", visible: false, width: 120, order: 10 },
  { id: "fwhm", label: "FWHM", visible: false, width: 75, order: 11 },
  { id: "starCount", label: "Stars", visible: false, width: 55, order: 12 },
  { id: "background", label: "BG", visible: false, width: 75, order: 13 },
  { id: "noise", label: "Noise", visible: false, width: 65, order: 14 },
  { id: "snr", label: "SNR", visible: false, width: 65, order: 15 },
  { id: "moonPhase", label: "Moon", visible: false, width: 65, order: 16 },
];

export const DEFAULT_COLUMNS: ColumnDef[] = [
  { id: "name", label: "Name", visible: true, width: 160, order: 0 },
  { id: "object", label: "Object", visible: true, width: 110, order: 1 },
  { id: "filter", label: "Filter", visible: true, width: 70, order: 2 },
  { id: "dateObs", label: "Date", visible: true, width: 130, order: 3 },
  { id: "expTime", label: "Exp", visible: true, width: 65, order: 4 },
  { id: "modTime", label: "Modified", visible: false, width: 140, order: 5 },
  { id: "size", label: "Size", visible: false, width: 70, order: 6 },
  { id: "gain", label: "Gain", visible: false, width: 60, order: 7 },
  { id: "ccdTemp", label: "Temp", visible: false, width: 75, order: 8 },
];
