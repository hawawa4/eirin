// ── Toasts ──────────────────────────────────────────────────────────────────
// Module-level notification queue rendered by <Toaster /> (mounted once in App).

export type ToastKind = "error" | "success" | "info";

export interface ToastAction {
  label: string;
  run: () => void | Promise<void>;
}

export interface ToastOptions {
  action?: ToastAction;
  /** Auto-dismiss delay in ms; 0 keeps the toast until dismissed. */
  timeout?: number;
}

export interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
  action?: ToastAction;
  timeout: number;
}

const ERROR_TIMEOUT = 8000;
const DEFAULT_TIMEOUT = 4000;
const ACTION_TIMEOUT = 6000;
const MAX_TOASTS = 5;

/** Reactive list of visible toasts (oldest first). Read-only for consumers. */
export const toasts: Toast[] = $state([]);

let nextId = 1;
// eslint-disable-next-line svelte/prefer-svelte-reactivity -- timer handles, intentionally non-reactive
const timers = new Map<number, ReturnType<typeof setTimeout>>();

function schedule(t: Toast) {
  clearTimer(t.id);
  if (t.timeout > 0)
    timers.set(
      t.id,
      setTimeout(() => dismiss(t.id), t.timeout),
    );
}

function clearTimer(id: number) {
  const h = timers.get(id);
  if (h !== undefined) clearTimeout(h);
  timers.delete(id);
}

function push(kind: ToastKind, message: string, opts: ToastOptions = {}): number {
  const fallback =
    kind === "error" ? ERROR_TIMEOUT : opts.action ? ACTION_TIMEOUT : DEFAULT_TIMEOUT;
  const t: Toast = {
    id: nextId++,
    kind,
    message,
    action: opts.action,
    timeout: opts.timeout ?? fallback,
  };
  toasts.push(t);
  while (toasts.length > MAX_TOASTS) dismiss(toasts[0].id);
  schedule(t);
  return t.id;
}

function dismiss(id: number): void {
  clearTimer(id);
  const i = toasts.findIndex((t) => t.id === id);
  if (i !== -1) toasts.splice(i, 1);
}

export const toast = {
  error: (msg: string, opts?: ToastOptions): number => push("error", msg, opts),
  success: (msg: string, opts?: ToastOptions): number => push("success", msg, opts),
  info: (msg: string, opts?: ToastOptions): number => push("info", msg, opts),
  dismiss,
  /** Stops the auto-dismiss timer (e.g. while hovered). */
  pause: (id: number): void => clearTimer(id),
  /** Restarts the auto-dismiss timer with the toast's full timeout. */
  resume: (id: number): void => {
    const t = toasts.find((x) => x.id === id);
    if (t) schedule(t);
  },
};

/**
 * Runs `fn`; on rejection shows `toast.error(`${errMsg}: ${e}`)` and resolves to undefined.
 */
export async function attempt<T>(fn: () => Promise<T>, errMsg: string): Promise<T | undefined> {
  try {
    return await fn();
  } catch (e) {
    toast.error(`${errMsg}: ${String(e)}`);
    return undefined;
  }
}
