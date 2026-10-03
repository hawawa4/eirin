// ── Squarified treemap ──────────────────────────────────────────────────────
// Bruls, Huizing & van Wijk (2000). Pure functions, no DOM.
//
// Items are sorted by value (descending) and laid out in "rows" along the
// shorter side of the remaining rectangle. A row keeps growing while adding the
// next item does not make its worst aspect ratio worse; the row then takes a
// strip of the remaining rectangle whose thickness is rowArea / shortSide, and
// the rest of the items are laid out in what is left.

export interface TreemapInput<T> {
  value: number;
  data: T;
}

export interface TreemapRect<T> {
  x: number;
  y: number;
  w: number;
  h: number;
  data: T;
}

/** Worst (largest) aspect ratio of a row of areas laid along a side of length `side`. */
function worstRatio(row: number[], side: number): number {
  if (row.length === 0 || side <= 0) return Infinity;
  let sum = 0;
  let max = -Infinity;
  let min = Infinity;
  for (const a of row) {
    sum += a;
    if (a > max) max = a;
    if (a < min) min = a;
  }
  if (sum <= 0 || min <= 0) return Infinity;
  const s2 = side * side;
  const sum2 = sum * sum;
  return Math.max((s2 * max) / sum2, sum2 / (s2 * min));
}

/**
 * Lays out `items` inside the rectangle (x, y, w, h). Items with a value <= 0 are
 * dropped (they would have no area). The returned rects exactly tile the
 * rectangle (up to floating-point error), ordered largest first.
 */
export function squarify<T>(
  items: TreemapInput<T>[],
  x: number,
  y: number,
  w: number,
  h: number,
): TreemapRect<T>[] {
  const nodes = items.filter((i) => i.value > 0).sort((a, b) => b.value - a.value);
  const total = nodes.reduce((s, n) => s + n.value, 0);
  if (nodes.length === 0 || total <= 0 || w <= 0 || h <= 0) return [];

  const scale = (w * h) / total;
  const areas = nodes.map((n) => n.value * scale);
  const out: TreemapRect<T>[] = [];

  let rx = x;
  let ry = y;
  let rw = w;
  let rh = h;
  let i = 0;

  while (i < nodes.length) {
    const side = Math.min(rw, rh);
    const row = [areas[i]];
    let j = i + 1;
    while (j < nodes.length && worstRatio([...row, areas[j]], side) <= worstRatio(row, side)) {
      row.push(areas[j]);
      j++;
    }
    const rowArea = row.reduce((s, a) => s + a, 0);
    const isLast = j >= nodes.length;

    if (rw >= rh) {
      // Short side is the height: the row is a column on the left.
      const colW = isLast ? rw : Math.min(rw, rowArea / rh);
      let cy = ry;
      for (let k = 0; k < row.length; k++) {
        const cellH = k === row.length - 1 ? ry + rh - cy : row[k] / colW;
        out.push({ x: rx, y: cy, w: colW, h: cellH, data: nodes[i + k].data });
        cy += cellH;
      }
      rx += colW;
      rw = Math.max(0, rw - colW);
    } else {
      // Short side is the width: the row is a strip along the top.
      const rowH = isLast ? rh : Math.min(rh, rowArea / rw);
      let cx = rx;
      for (let k = 0; k < row.length; k++) {
        const cellW = k === row.length - 1 ? rx + rw - cx : row[k] / rowH;
        out.push({ x: cx, y: ry, w: cellW, h: rowH, data: nodes[i + k].data });
        cx += cellW;
      }
      ry += rowH;
      rh = Math.max(0, rh - rowH);
    }
    i = j;
  }
  return out;
}

/**
 * Sanity check for a layout: every rect lies inside the bounds, rects do not
 * overlap, their areas sum to w*h, and each area is proportional to its value.
 * Returns a list of problems (empty = OK).
 */
export function checkLayout<T>(
  rects: TreemapRect<T>[],
  valueOf: (d: T) => number,
  w: number,
  h: number,
  eps = 1e-6,
): string[] {
  const problems: string[] = [];
  const total = rects.reduce((s, r) => s + valueOf(r.data), 0);
  let area = 0;
  rects.forEach((r, idx) => {
    area += r.w * r.h;
    if (r.x < -eps || r.y < -eps || r.x + r.w > w + eps || r.y + r.h > h + eps)
      problems.push(`rect ${idx} out of bounds`);
    const want = (valueOf(r.data) / total) * w * h;
    if (Math.abs(r.w * r.h - want) > Math.max(eps, want * 1e-6))
      problems.push(`rect ${idx} area ${r.w * r.h} != ${want}`);
    for (let j = idx + 1; j < rects.length; j++) {
      const o = rects[j];
      const ix = Math.min(r.x + r.w, o.x + o.w) - Math.max(r.x, o.x);
      const iy = Math.min(r.y + r.h, o.y + o.h) - Math.max(r.y, o.y);
      if (ix > eps && iy > eps) problems.push(`rects ${idx} and ${j} overlap`);
    }
  });
  if (Math.abs(area - w * h) > eps * w * h) problems.push(`areas sum to ${area}, not ${w * h}`);
  return problems;
}

/**
 * Self-check on the example from the paper: values 6,6,4,3,2,2,1 in a 6×4 box.
 * Expected: first column 3×4 holding the two 6s (each 3×2), then the rest.
 */
export function selfCheck(): string[] {
  const values = [6, 6, 4, 3, 2, 2, 1];
  const rects = squarify(
    values.map((v) => ({ value: v, data: v })),
    0,
    0,
    6,
    4,
  );
  const problems = checkLayout(rects, (d) => d, 6, 4);
  if (rects.length !== values.length) problems.push(`expected ${values.length} rects`);
  const [a, b] = rects;
  if (!a || Math.abs(a.w - 3) > 1e-9 || Math.abs(a.h - 2) > 1e-9)
    problems.push("first rect should be 3×2");
  if (!b || Math.abs(b.x) > 1e-9 || Math.abs(b.y - 2) > 1e-9)
    problems.push("second rect should sit below the first");
  return problems;
}

if (import.meta.env?.DEV) {
  const problems = selfCheck();
  if (problems.length) console.warn("treemap self-check failed:", problems);
}
