// ── Native context menu guard ───────────────────────────────────────────────
// Eirin has its own right-click menus; the WebView's (Reload / Inspect Element)
// should never show up. Wails' runtime already hides it in production builds,
// but debug builds (`just dev`) always show it, so this runs in both.

const NON_TEXT_INPUTS = new Set([
  "checkbox",
  "radio",
  "button",
  "submit",
  "reset",
  "range",
  "color",
  "file",
  "image",
]);

/** Text fields keep the native menu: it's their Cut / Copy / Paste. */
export function isEditableTarget(t: EventTarget | null): boolean {
  if (t instanceof HTMLTextAreaElement) return !t.disabled;
  if (t instanceof HTMLInputElement) return !t.disabled && !NON_TEXT_INPUTS.has(t.type);
  return t instanceof HTMLElement && t.isContentEditable;
}

export function installContextMenuGuard(): () => void {
  function onContextMenu(e: MouseEvent) {
    // Dev escape hatch: Shift+right-click still reaches Inspect Element.
    if (import.meta.env.DEV && e.shiftKey) return;
    if (isEditableTarget(e.target)) return;
    // Only blocks the native menu; in-app menus still get the event.
    e.preventDefault();
  }
  window.addEventListener("contextmenu", onContextMenu, true);
  return () => window.removeEventListener("contextmenu", onContextMenu, true);
}
