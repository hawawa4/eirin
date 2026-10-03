// ── Modal stack ─────────────────────────────────────────────────────────────
// Tracks open <Modal> instances so only the top-most one reacts to Escape/Tab,
// and so view-level keyboard shortcuts can stand down while any modal is open.

const stack: symbol[] = [];

/** Registers a newly opened modal and returns its handle. */
export function pushModal(): symbol {
  const id = Symbol("modal");
  stack.push(id);
  return id;
}

/** Unregisters a modal (safe to call for an id that is not on top). */
export function popModal(id: symbol): void {
  const i = stack.lastIndexOf(id);
  if (i !== -1) stack.splice(i, 1);
}

/** True if `id` is the top-most open modal. */
export function isTopModal(id: symbol): boolean {
  return stack.length > 0 && stack[stack.length - 1] === id;
}

/** True while at least one modal is open. */
export function isModalOpen(): boolean {
  return stack.length > 0;
}
