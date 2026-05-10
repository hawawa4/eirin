import type { browser, app } from "../../wailsjs/go/models";

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
  files: browser.EnrichedFileEntry[];
}

export interface IndexProgress {
  phase: "scanning" | "indexing" | "done" | "cancelled";
  total: number;
  done: number;
  indexed: number;
  errors: number;
  current: string;
}

export interface CtxMenuState {
  x: number;
  y: number;
  entry: browser.EnrichedFileEntry;
}

export type ViewMode = "files" | "rejected";

export type AppMode = "browser" | "library";

export type FrameType = "light" | "dark" | "flat" | "bias" | "stacked" | "processed";

export type LibraryGroupBy = "object" | "date" | "filter" | "frameType";

export interface LibraryGroup {
  key: string;
  label: string;
  frames: app.LibraryFrame[];
}

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
