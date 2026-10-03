// ── Canvas label placement with collision avoidance ─────────────────────────
// Labels are placed first-come, first-served each redraw: earlier calls
// (higher-priority content) claim screen space, later ones are skipped if they
// would overlap. Call reset() at the top of every redraw.

// All canvas label fonts are ≥ 12px for readability.
export const LABEL_FONT = "600 12px system-ui, sans-serif";
export const LABEL_FONT_SMALL = "500 12px system-ui, sans-serif";
export const LABEL_FONT_EMPHASIS = "700 13px system-ui, sans-serif";

const LABEL_BG = "rgba(6,8,18,0.82)";

export interface LabelOptions {
  font?: string;
  color: string;
  align?: CanvasTextAlign;
  bg?: boolean;
  /** Draw even if it overlaps (still reserves its space). */
  force?: boolean;
}

export class LabelPlacer {
  private placed: { x: number; y: number; w: number; h: number }[] = [];

  reset(): void {
    this.placed.length = 0;
  }

  private overlaps(x: number, y: number, w: number, h: number): boolean {
    const pad = 2;
    for (const r of this.placed) {
      if (x - pad < r.x + r.w && x + w + pad > r.x && y - pad < r.y + r.h && y + h + pad > r.y) {
        return true;
      }
    }
    return false;
  }

  /** Draws `text` with its baseline at (x, y) unless it collides. Returns whether it drew. */
  place(
    ctx: CanvasRenderingContext2D,
    text: string,
    x: number,
    y: number,
    opts: LabelOptions,
  ): boolean {
    ctx.font = opts.font ?? LABEL_FONT;
    const align = opts.align ?? "left";
    const tw = ctx.measureText(text).width;
    const boxX = align === "center" ? x - tw / 2 : align === "right" ? x - tw : x;
    const boxY = y - 13;
    const boxW = tw;
    const boxH = 17;

    if (!opts.force && this.overlaps(boxX, boxY, boxW, boxH)) return false;
    this.placed.push({ x: boxX, y: boxY, w: boxW, h: boxH });

    if (opts.bg) {
      ctx.fillStyle = LABEL_BG;
      roundRect(ctx, boxX - 4, boxY, boxW + 8, boxH, 3);
      ctx.fill();
    }
    ctx.textAlign = align;
    ctx.fillStyle = opts.color;
    ctx.fillText(text, x, y);
    ctx.textAlign = "left";
    return true;
  }
}

export function roundRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
): void {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}
