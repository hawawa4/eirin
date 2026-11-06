import { formatDuration, type FrameInfo } from "../lib/slideshow/frameInfo";
import type { GalleryImage } from "./types";

/** "2024-01-01" as a calendar day, whatever the viewer's time zone. */
export function formatDate(iso: string): string {
  const m = iso.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!m) return iso;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3])).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

/** "30 × 10 s = 5 min", or just the total when the subs aren't known. */
export function exposureText(img: GalleryImage): string | undefined {
  if (img.subs > 0 && img.subExposure > 0) {
    return `${img.subs} × ${formatDuration(img.subExposure)} = ${formatDuration(img.exposure)}`;
  }
  return img.exposure > 0 ? formatDuration(img.exposure) : undefined;
}

/** The capture details shown with a picture, from what the server worked out. */
export function describeImage(img: GalleryImage): FrameInfo {
  const rows: FrameInfo["rows"] = [];
  if (img.telescope) rows.push(["Telescope", img.telescope]);
  const exposure = exposureText(img);
  if (exposure) rows.push(["Exposure", exposure]);
  if (img.date) rows.push(["Date", formatDate(img.date)]);
  if (img.filter) rows.push(["Filter", img.filter]);
  return { title: img.title, rows };
}

/** One line for a card: "Jan 1, 2024 · 5 min". */
export function summaryLine(img: GalleryImage): string {
  const parts = [
    img.date ? formatDate(img.date) : "",
    img.exposure > 0 ? formatDuration(img.exposure) : "",
  ];
  return parts.filter(Boolean).join(" · ");
}
