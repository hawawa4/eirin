import type { app } from "../../wailsjs/go/models";

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
}

export interface CtxMenuState {
  x: number;
  y: number;
  entry: CtxEntry;
}

export type ViewMode = "files" | "rejected";

export type AppMode = "browser" | "library" | "import";

export type FrameType = "light" | "dark" | "flat" | "bias" | "stacked" | "processed";

export type LibraryGroupBy = "object" | "date" | "filter" | "frameType";

export type ImportState = "idle" | "scanning" | "scanned" | "importing" | "done" | "error";

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

export interface LibraryGroup {
  key: string;
  label: string;
  frames: app.LibraryFrame[];
}

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
