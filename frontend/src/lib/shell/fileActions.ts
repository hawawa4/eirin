// ── OS file-manager / clipboard helpers with toast feedback ────────────────

import { OpenFolder, RevealPath } from "$app";
import { attempt, toast } from "../toast.svelte";

/** Opens the OS file manager with `path` selected. */
export async function revealPath(path: string): Promise<void> {
  await attempt(() => RevealPath(path), "Couldn't show in folder");
}

/** Opens `path` (a folder, or a file's parent folder) in the OS file manager. */
export async function openFolder(path: string): Promise<void> {
  await attempt(() => OpenFolder(path), "Couldn't open folder");
}

export async function copyText(text: string, successMsg = "Copied"): Promise<void> {
  try {
    await navigator.clipboard.writeText(text);
    toast.success(successMsg);
  } catch (e) {
    toast.error(`Couldn't copy to clipboard: ${String(e)}`);
  }
}

export function copyPath(path: string): Promise<void> {
  return copyText(path, "Path copied");
}
