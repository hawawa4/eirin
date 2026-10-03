// ── Import session ──────────────────────────────────────────────────────────
// Module-level state for the Import view, so a running (or finished) import,
// the scanned source and the tree's collapsed folders survive the view being
// unmounted when the user switches tabs. The backend is the source of truth for
// the job itself: on mount the view calls syncStatus() to pick it back up.

import { SvelteSet } from "svelte/reactivity";
import { Events } from "@wailsio/runtime";
import {
  CancelImport,
  GetImportStatus,
  ScanImportCandidates,
  SelectSourceFolder,
  StartImport,
} from "$app";
import type * as importer from "$models/importer";
import { toast } from "../toast.svelte";

export type Progress = importer.Progress;
export type Candidate = importer.Candidate;

/** pick: choose a source · scanning: first scan · review: candidate tree · running/result: the job. */
export type ImportScreen = "pick" | "scanning" | "review" | "running" | "result";

export const FORMAT_GROUPS: readonly { key: string; label: string; exts: string[] }[] = [
  { key: "fits", label: "FITS", exts: ["fit", "fits"] },
  { key: "png", label: "PNG", exts: ["png"] },
  { key: "jpeg", label: "JPEG", exts: ["jpg", "jpeg"] },
  { key: "tiff", label: "TIFF", exts: ["tif", "tiff"] },
];

const RUNNING_PHASES = new Set(["scanning", "copying"]);
const FINISHED_PHASES = new Set(["done", "cancelled", "error"]);

export function isRunningPhase(phase: string | undefined): boolean {
  return !!phase && RUNNING_PHASES.has(phase);
}

export const session = $state({
  screen: "pick" as ImportScreen,
  sourceFolder: "",
  candidates: [] as Candidate[],
  /** Error from the last ScanImportCandidates call. */
  scanError: "",
  /** A re-scan (e.g. after toggling a format) is in flight; the tree stays visible. */
  rescanning: false,
  deleteAfterCopy: false,
  progress: null as Progress | null,
  /** StartImport call in flight. */
  starting: false,
  /** StartImport rejected (job already running, root inaccessible, scan failed…). */
  startError: "",
  cancelling: false,
  /** The user left the result screen ("Import more"); don't bring it back on remount. */
  resultDismissed: false,
});

export const selectedFormats = new SvelteSet<string>(["fits"]);
export const collapsedFolders = new SvelteSet<string>();

export function extensions(): string[] {
  return FORMAT_GROUPS.filter((g) => selectedFormats.has(g.key)).flatMap((g) => g.exts);
}

// ── Progress ──────────────────────────────────────────────────────────────────

function applyProgress(p: Progress) {
  session.progress = p;
  if (RUNNING_PHASES.has(p.phase)) {
    session.resultDismissed = false;
    session.screen = "running";
  } else if (FINISHED_PHASES.has(p.phase)) {
    session.cancelling = false;
    if (!session.resultDismissed) session.screen = "result";
  }
}

let listening = false;

/** Subscribes (once, for the app's lifetime) to "import:progress". */
export function ensureListening(): void {
  if (listening) return;
  listening = true;
  Events.On("import:progress", (event) => {
    if (event.data) applyProgress(event.data as Progress);
  });
}

/** Re-reads the job state from the backend (after a remount or a missed event). */
export async function syncStatus(): Promise<void> {
  try {
    const st = await GetImportStatus();
    if (!st || st.phase === "idle" || !st.phase) return;
    if (RUNNING_PHASES.has(st.phase) || (FINISHED_PHASES.has(st.phase) && !session.resultDismissed))
      applyProgress(st);
  } catch {
    /* status is best-effort; events will still arrive */
  }
}

// ── Scanning ──────────────────────────────────────────────────────────────────

let scanToken = 0;

/** Scans the source. `initial` shows the full-screen scanning state; otherwise the tree stays. */
export async function scan(initial: boolean): Promise<void> {
  if (!session.sourceFolder) return;
  const token = ++scanToken;
  session.scanError = "";
  if (initial) session.screen = "scanning";
  else session.rescanning = true;
  try {
    const result = await ScanImportCandidates(session.sourceFolder, extensions());
    if (token !== scanToken) return;
    session.candidates = result ?? [];
    session.screen = "review";
  } catch (e) {
    if (token !== scanToken) return;
    session.scanError = String(e);
    if (initial) session.screen = "pick";
  } finally {
    if (token === scanToken) session.rescanning = false;
  }
}

export async function chooseSource(): Promise<void> {
  let path: string;
  try {
    path = await SelectSourceFolder();
  } catch (e) {
    toast.error(`Couldn't open the folder picker: ${String(e)}`);
    return;
  }
  if (!path) return;
  session.sourceFolder = path;
  session.startError = "";
  collapsedFolders.clear();
  await scan(true);
}

export function toggleFormat(key: string): void {
  if (selectedFormats.has(key)) selectedFormats.delete(key);
  else selectedFormats.add(key);
  if (session.screen !== "review") return;
  if (selectedFormats.size === 0) {
    // An empty extension list means "every file" to the backend: show nothing instead.
    scanToken++;
    session.rescanning = false;
    session.candidates = [];
    return;
  }
  void scan(false);
}

// ── Running ───────────────────────────────────────────────────────────────────

export async function startImport(): Promise<void> {
  if (session.starting || isRunningPhase(session.progress?.phase)) return;
  session.starting = true;
  session.startError = "";
  session.progress = null;
  session.resultDismissed = false;
  try {
    await StartImport(session.sourceFolder, extensions(), session.deleteAfterCopy);
    // Events may already have moved us to running/result; otherwise show the job starting.
    if (!session.progress) {
      session.progress = {
        phase: "scanning",
        current: 0,
        total: 0,
        currentFile: "",
        copied: 0,
        skipped: 0,
        kept: 0,
        errors: [],
        notes: [],
      };
      session.screen = "running";
    }
  } catch (e) {
    session.startError = String(e);
    session.resultDismissed = true;
    // Most likely another import is already running: show it.
    try {
      const st = await GetImportStatus();
      if (st && RUNNING_PHASES.has(st.phase)) applyProgress(st);
    } catch {
      /* keep the start error */
    }
  } finally {
    session.starting = false;
  }
}

export async function cancelImport(): Promise<void> {
  session.cancelling = true;
  try {
    await CancelImport();
  } catch (e) {
    session.cancelling = false;
    toast.error(`Couldn't cancel the import: ${String(e)}`);
  }
}

/** Leaves the result screen and starts over. */
export function resetSession(): void {
  session.resultDismissed = true;
  session.screen = "pick";
  session.sourceFolder = "";
  session.candidates = [];
  session.scanError = "";
  session.startError = "";
  session.progress = null;
  collapsedFolders.clear();
}
