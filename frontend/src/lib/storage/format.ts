// ── Storage view formatting helpers ────────────────────────────────────────

/** Human-readable byte size (decimal units, like file managers). */
export function fmtBytes(b: number): string {
  if (b >= 1e12) return (b / 1e12).toFixed(1) + " TB";
  if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB";
  if (b >= 1e6) return (b / 1e6).toFixed(1) + " MB";
  if (b >= 1e3) return (b / 1e3).toFixed(0) + " KB";
  return b + " B";
}

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
