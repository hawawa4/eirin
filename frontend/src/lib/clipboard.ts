// ── Clipboard helpers with toast feedback ──────────────────────────────────

import { Clipboard } from "@wailsio/runtime";
import { toast } from "./toast.svelte";

/**
 * Copies `text` to the clipboard and confirms with a toast. Uses the Wails
 * runtime clipboard first (navigator.clipboard is often unavailable in the
 * WebKitGTK webview), falling back to the browser API (e.g. in server mode).
 */
export async function copyText(text: string, successMsg = "Copied"): Promise<void> {
  try {
    await Clipboard.SetText(text);
  } catch {
    try {
      await navigator.clipboard.writeText(text);
    } catch (e) {
      toast.error(`Couldn't copy to the clipboard: ${String(e)}`);
      return;
    }
  }
  toast.success(successMsg);
}

export function copyPath(path: string): Promise<void> {
  return copyText(path, "Path copied");
}

/** Copies one path per line. */
export function copyPaths(paths: string[]): Promise<void> {
  return copyText(
    paths.join("\n"),
    paths.length > 1 ? `${paths.length} paths copied` : "Path copied",
  );
}
