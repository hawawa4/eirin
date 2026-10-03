// ── Library index scans ─────────────────────────────────────────────────────
// Owns the "index:progress" event stream and the start/cancel calls, so App only
// wires triggers (buttons, root change) and reacts to completion. Imports index
// their copied files themselves, so they never trigger a scan.

import { BuildIndex, CancelIndex } from "$app";
import { Events } from "@wailsio/runtime";
import type { IndexProgress } from "../types";
import { toast } from "../toast.svelte";

/** index:progress payload — the backend adds `error` on a failed run (phase "done"). */
export interface IndexProgressEvent extends IndexProgress {
  error?: string;
}

export type IndexOutcome =
  | { kind: "done"; progress: IndexProgressEvent }
  | { kind: "cancelled"; progress: IndexProgressEvent }
  | { kind: "error"; progress: IndexProgressEvent | null; message: string };

export interface StartOptions {
  force?: boolean;
  ifRunning?: "ignore" | "supersede";
}

function isFinished(p: IndexProgressEvent | null): boolean {
  return p === null || p.phase === "done" || p.phase === "cancelled";
}

class IndexController {
  progress = $state<IndexProgressEvent | null>(null);
  running = $derived(!isFinished(this.progress));

  #onfinish: ((o: IndexOutcome) => void) | null = null;

  /**
   * Starts a scan of `root` (`force` re-reads already-indexed files). `ifRunning`
   * decides what happens when a scan is already active: "ignore" drops the request,
   * "supersede" starts it right away — the backend cancels the old run.
   */
  async start(
    root: string,
    { force = false, ifRunning = "ignore" }: StartOptions = {},
  ): Promise<void> {
    if (!root) return;
    if (this.running && ifRunning !== "supersede") return;
    this.progress = {
      phase: "scanning",
      total: 0,
      done: 0,
      indexed: 0,
      errors: 0,
      current: "Starting…",
    };
    try {
      await BuildIndex(root, force);
    } catch (e) {
      this.progress = null;
      this.#finish({ kind: "error", progress: null, message: String(e) });
    }
  }

  async cancel(): Promise<void> {
    try {
      await CancelIndex();
    } catch (e) {
      toast.error(`Couldn't cancel the scan: ${String(e)}`);
    }
  }

  /** Subscribes to backend progress; `onfinish` runs once per finished run. Returns unsubscribe. */
  listen(onfinish: (o: IndexOutcome) => void): () => void {
    this.#onfinish = onfinish;
    const off = Events.On("index:progress", (event) => {
      const data = event.data as IndexProgressEvent;
      this.progress = data;
      if (data.error) {
        this.#finish({ kind: "error", progress: data, message: data.error });
      } else if (data.phase === "done") {
        this.#finish({ kind: "done", progress: data });
      } else if (data.phase === "cancelled") {
        this.#finish({ kind: "cancelled", progress: data });
      }
    });
    return () => {
      off();
      this.#onfinish = null;
    };
  }

  #finish(o: IndexOutcome) {
    // An error event may arrive with a non-final phase; make sure the UI unlocks.
    if (o.kind === "error" && o.progress && !isFinished(o.progress)) {
      this.progress = { ...o.progress, phase: "done" };
    }
    this.#onfinish?.(o);
  }
}

export const indexer = new IndexController();
