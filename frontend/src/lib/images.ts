// ── Display images ──────────────────────────────────────────────────────────
// Previews are plain JPEGs served by the backend on its own HTTP route
// (internal/app/images.go), so the browser fetches them as binary, decodes
// them off the main thread and caches them — no base64 inside RPC replies.
// They come in a few fixed sizes (a pyramid rendered once per file and kept
// on disk), so a view asks for the size it shows the image at, and asks
// again for a sharper one when the user zooms in.

/** Longest-side sizes previews come in, smallest first (internal/previews.Levels). */
export const LEVELS = [512, 1024, 2048, 4096] as const;
/** The image at its own size (capped at 8192 px by the backend). */
export const NATIVE = 0;

const ROUTE = "/eirin/preview";

/** The level serving a request for `px` pixels on the longest side (≤ 0: native). */
export function snapLevel(px: number): number {
  if (!(px > 0)) return NATIVE;
  for (const l of LEVELS) if (px <= l) return l;
  return NATIVE;
}

/**
 * The level that shows an image sharply when drawn `cssPx` CSS pixels on its
 * longest side: device pixels, so HiDPI screens get the next level up.
 */
export function levelFor(cssPx: number, dpr = window.devicePixelRatio || 1): number {
  return snapLevel(Math.ceil(cssPx * dpr));
}

/** Orders levels by pixel size (native is the largest). */
export function levelRank(level: number): number {
  return level === NATIVE ? Infinity : level;
}

/**
 * The stretch a frame is first shown at: finished pictures as they are
 * (the server pre-renders those at 0, see warmPreviews in
 * internal/app/startup_server.go), stacks with the normal autostretch.
 */
export function stretchFor(frameType: string | undefined): number {
  return frameType === "processed" || frameType === "image" ? 0 : 2;
}

/** URL of `path` at `level`, stretched at `stretch` (FITS only; 0–3). */
export function previewUrl(path: string, level: number, stretch = 0): string {
  const q = new URLSearchParams({ path, size: String(level), stretch: String(stretch) });
  return `${ROUTE}?${q.toString()}`;
}

/** Fetches and decodes a preview off the main thread. */
export async function fetchBitmap(url: string, signal?: AbortSignal): Promise<ImageBitmap> {
  const res = await fetch(url, { signal });
  if (!res.ok) {
    const msg = (await res.text().catch(() => "")).trim();
    throw new Error(msg || `HTTP ${res.status}`);
  }
  // No options: the server's JPEGs carry no EXIF orientation to apply.
  return createImageBitmap(await res.blob());
}

/**
 * Loads `url` into an <img> and waits until it's decoded, so showing it
 * afterwards doesn't stutter. Kept elements stay decoded in the browser's
 * image cache.
 */
export async function loadImage(url: string): Promise<HTMLImageElement> {
  const img = new Image();
  img.decoding = "async";
  img.src = url;
  try {
    await img.decode();
  } catch {
    // decode() rejects without a reason; ask the server what went wrong.
    const res = await fetch(url).catch(() => null);
    const msg = res && !res.ok ? (await res.text().catch(() => "")).trim() : "";
    throw new Error(msg || "couldn't load the image");
  }
  return img;
}
