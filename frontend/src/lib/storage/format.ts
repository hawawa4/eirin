// ── Storage view formatting helpers ────────────────────────────────────────

export function fmtFrames(n: number): string {
  return `${n.toLocaleString()} frame${n === 1 ? "" : "s"}`;
}

/** Rough width of `text` in px for a proportional UI font of `fontPx` size. */
export function estimateTextWidth(text: string, fontPx: number): number {
  return text.length * fontPx * 0.6;
}

/**
 * Returns `text`, or a shortened "text…" that fits `maxWidth`, or null if not
 * even a few characters fit (the label is then hidden rather than shrunk).
 */
export function fitLabel(text: string, maxWidth: number, fontPx: number): string | null {
  if (estimateTextWidth(text, fontPx) <= maxWidth) return text;
  const chars = Math.floor(maxWidth / (fontPx * 0.6)) - 1;
  if (chars < 4) return null;
  return text.slice(0, chars) + "…";
}
