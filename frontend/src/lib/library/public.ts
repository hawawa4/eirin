import type * as app from "$models/app";

/** Only finished pictures can go in the public gallery. */
export function canBePublic(frameType: string): boolean {
  return frameType === "processed" || frameType === "image";
}

/** The frames among `paths` that can go in the public gallery. */
export function publishable(
  paths: readonly string[],
  byPath: ReadonlyMap<string, app.LibraryFrame>,
): app.LibraryFrame[] {
  const out: app.LibraryFrame[] = [];
  for (const p of paths) {
    const f = byPath.get(p);
    if (f && canBePublic(f.frameType) && !f.isRejected) out.push(f);
  }
  return out;
}
