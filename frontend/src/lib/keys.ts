// ── Keyboard helpers ────────────────────────────────────────────────────────

export { isModalOpen } from "./modalStack";

/** True if the event originated in an editable control (input, textarea, select, contenteditable). */
export function isTypingTarget(e: Event): boolean {
  const t = e.target;
  if (!(t instanceof HTMLElement)) return false;
  if (t.isContentEditable) return true;
  const tag = t.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}
