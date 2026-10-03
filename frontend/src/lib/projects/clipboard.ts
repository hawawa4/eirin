import { Clipboard } from "@wailsio/runtime";
import { toast } from "../toast.svelte";

/** Copies `text` to the clipboard and confirms with a toast. */
export async function copyText(text: string, what = "Path"): Promise<void> {
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
  toast.success(`${what} copied to the clipboard`);
}
