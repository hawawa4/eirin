<script lang="ts">
  import type * as app from "$models/app";
  import { GetStorageStats, GetFrameTypeSummary } from "$app";

  interface Props {
    rootPath: string;
  }

  let { rootPath }: Props = $props();

  // ── Data ──────────────────────────────────────────────────────────────────
  let root = $state<app.StorageNode | null>(null);
  let typeSummary = $state<app.StorageNode[]>([]);
  let loading = $state(false);
  let error = $state("");

  // ── Drill-down: null = show root children, string = show that object's children
  let drillObject = $state<string | null>(null);

  $effect(() => {
    if (!rootPath) return;
    loading = true;
    error = "";
    Promise.all([GetStorageStats(rootPath), GetFrameTypeSummary(rootPath)])
      .then(([r, t]) => {
        root = r;
        typeSummary = t ?? [];
        loading = false;
      })
      .catch((e) => {
        error = String(e);
        loading = false;
      });
  });

  // ── Treemap layout ────────────────────────────────────────────────────────
  interface Rect {
    x: number;
    y: number;
    w: number;
    h: number;
    node: app.StorageNode;
  }

  function squarify(nodes: app.StorageNode[], x: number, y: number, w: number, h: number): Rect[] {
    if (!nodes.length) return [];
    const total = nodes.reduce((s, n) => s + n.totalBytes, 0);
    if (total === 0) return [];

    const rects: Rect[] = [];
    let remaining = [...nodes];
    let rx = x,
      ry = y,
      rw = w,
      rh = h;

    while (remaining.length > 0) {
      const rowArea = rw * rh;
      const row: app.StorageNode[] = [];
      let rowTotal = 0;

      for (const node of remaining) {
        const candidate = [...row, node];
        const candidateTotal = rowTotal + node.totalBytes;
        const short = Math.min(rw, rh);
        const ar = worstAR(candidate, candidateTotal, rowArea, short);
        if (row.length === 0 || ar <= worstAR(row, rowTotal, rowArea, short)) {
          row.push(node);
          rowTotal += node.totalBytes;
        } else {
          break;
        }
      }

      remaining = remaining.slice(row.length);

      // Layout the row
      const rowFrac = rowTotal / total;
      let cx = rx,
        cy = ry;
      if (rw <= rh) {
        // horizontal strip
        const stripH =
          rh *
          (rowTotal /
            (total -
              (nodes.reduce((s, n) => s + n.totalBytes, 0) -
                rowTotal -
                remaining.reduce((s, n) => s + n.totalBytes, 0))));
        const fixedH = (rowTotal / total) * (rw <= rh ? rh : rw);
        for (const node of row) {
          const frac = node.totalBytes / rowTotal;
          rects.push({
            x: cx,
            y: cy,
            w: rw * (rw > rh ? 1 : frac),
            h: rw > rh ? frac * rh : fixedH,
            node,
          });
          if (rw > rh) cx += rw * frac;
          else cy += fixedH; // wrong but let's simplify
        }
        void rowFrac;
        void stripH;
        ry += fixedH;
        rh -= fixedH;
      } else {
        const fixedW = (rowTotal / total) * rw;
        for (const node of row) {
          const frac = node.totalBytes / rowTotal;
          rects.push({ x: cx, y: cy, w: fixedW, h: frac * rh, node });
          cy += frac * rh;
        }
        rx += fixedW;
        rw -= fixedW;
      }
    }

    return rects;
  }

  function worstAR(nodes: app.StorageNode[], total: number, area: number, short: number): number {
    if (!nodes.length || total === 0) return Infinity;
    let worst = 0;
    for (const n of nodes) {
      const frac = n.totalBytes / total;
      const rectArea = area * frac;
      const side = rectArea / short;
      const ar = Math.max(short / side, side / short);
      if (ar > worst) worst = ar;
    }
    return worst;
  }

  // ── Human-readable bytes ──────────────────────────────────────────────────
  function fmtBytes(b: number): string {
    if (b >= 1e12) return (b / 1e12).toFixed(1) + " TB";
    if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB";
    if (b >= 1e6) return (b / 1e6).toFixed(1) + " MB";
    if (b >= 1e3) return (b / 1e3).toFixed(0) + " KB";
    return b + " B";
  }

  // ── Colors ────────────────────────────────────────────────────────────────
  const PALETTE = [
    "#1e4e8c",
    "#2e6ca6",
    "#3a85bb",
    "#1a6b5a",
    "#21875e",
    "#5a3a8c",
    "#7b3db8",
    "#8c4a1a",
    "#b06020",
    "#6b1a1a",
    "#8c1a6b",
    "#1a4e6b",
    "#2a6e3a",
    "#4e3a8c",
    "#6b5a1a",
  ];
  function nodeColor(label: string, idx: number): string {
    // hash the label for a stable color assignment
    let hash = 5381;
    for (let i = 0; i < label.length; i++) hash = (hash * 33) ^ label.charCodeAt(i);
    return PALETTE[Math.abs(hash) % PALETTE.length] ?? PALETTE[idx % PALETTE.length];
  }

  // ── Derived treemap data ──────────────────────────────────────────────────
  const TW = 720,
    TH = 400;

  let treemapNodes = $derived.by(() => {
    if (!root) return [];
    const src = drillObject
      ? (root.children?.find((c) => c.label === drillObject)?.children ?? [])
      : (root.children ?? []);
    if (!src.length) return [];
    return squarify(src, 0, 0, TW, TH);
  });

  let hovered = $state<string | null>(null);
</script>

<div class="storage-view">
  <div class="storage-header">
    <div class="storage-title-row">
      <h2 class="storage-title">Storage</h2>
      {#if root}
        <span class="storage-total">{fmtBytes(root.totalBytes)} · {root.frameCount} frames</span>
      {/if}
    </div>

    {#if drillObject}
      <button class="breadcrumb-btn" onclick={() => (drillObject = null)}> ← All objects </button>
    {/if}
  </div>

  {#if loading}
    <div class="storage-status">Loading storage data…</div>
  {:else if error}
    <div class="storage-status error">{error}</div>
  {:else if !root || !root.children?.length}
    <div class="storage-status">No indexed frames found in this library.</div>
  {:else}
    <!-- Type summary bar -->
    {#if !drillObject && typeSummary.length > 0}
      <div class="type-bar">
        {#each typeSummary as t (t.label)}
          <div class="type-chip">
            <span class="type-label">{t.label}</span>
            <span class="type-size">{fmtBytes(t.totalBytes)}</span>
            <span class="type-count">{t.frameCount}</span>
          </div>
        {/each}
      </div>
    {/if}

    <!-- Treemap -->
    <div class="treemap-container">
      <svg width={TW} height={TH} class="treemap-svg">
        {#each treemapNodes as rect, i (rect.node.label)}
          {@const label = rect.node.label}
          {@const color = nodeColor(label, i)}
          {@const isHovered = hovered === label}
          <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
          <g
            class="treemap-cell"
            onmouseenter={() => (hovered = label)}
            onmouseleave={() => (hovered = null)}
            onclick={() => {
              if (!drillObject && rect.node.children?.length) drillObject = label;
            }}
            onkeydown={(e) => {
              if (
                (e.key === "Enter" || e.key === " ") &&
                !drillObject &&
                rect.node.children?.length
              )
                drillObject = label;
            }}
            role={rect.node.children?.length && !drillObject ? "button" : "img"}
            tabindex={rect.node.children?.length && !drillObject ? 0 : undefined}
            aria-label={label}
            style="cursor: {!drillObject && rect.node.children?.length ? 'pointer' : 'default'}"
          >
            <rect
              x={rect.x + 1}
              y={rect.y + 1}
              width={Math.max(0, rect.w - 2)}
              height={Math.max(0, rect.h - 2)}
              fill={color}
              fill-opacity={isHovered ? 0.95 : 0.75}
              stroke={isHovered ? "var(--accent)" : "rgba(255,255,255,0.08)"}
              stroke-width={isHovered ? 1.5 : 0.5}
              rx="3"
            />
            {#if rect.w > 60 && rect.h > 36}
              <text
                x={rect.x + rect.w / 2}
                y={rect.y + rect.h / 2 - 7}
                text-anchor="middle"
                dominant-baseline="middle"
                class="tm-label">{label}</text
              >
              <text
                x={rect.x + rect.w / 2}
                y={rect.y + rect.h / 2 + 9}
                text-anchor="middle"
                dominant-baseline="middle"
                class="tm-sub">{fmtBytes(rect.node.totalBytes)}</text
              >
              {#if rect.h > 56}
                <text
                  x={rect.x + rect.w / 2}
                  y={rect.y + rect.h / 2 + 23}
                  text-anchor="middle"
                  dominant-baseline="middle"
                  class="tm-sub">{rect.node.frameCount} frames</text
                >
              {/if}
            {:else if rect.w > 28 && rect.h > 20}
              <text
                x={rect.x + rect.w / 2}
                y={rect.y + rect.h / 2}
                text-anchor="middle"
                dominant-baseline="middle"
                class="tm-label-sm">{label.length > 14 ? label.slice(0, 12) + "…" : label}</text
              >
            {/if}
          </g>
        {/each}
      </svg>

      {#if hovered}
        {@const hNode = drillObject
          ? root?.children
              ?.find((c) => c.label === drillObject)
              ?.children?.find((c) => c.label === hovered)
          : root?.children?.find((c) => c.label === hovered)}
        {#if hNode}
          <div class="treemap-tooltip">
            <strong>{hNode.label}</strong>
            <span>{fmtBytes(hNode.totalBytes)}</span>
            <span>{hNode.frameCount} frame{hNode.frameCount !== 1 ? "s" : ""}</span>
            {#if !drillObject && hNode.children?.length}
              <span class="tooltip-hint">Click to drill down</span>
            {/if}
          </div>
        {/if}
      {/if}
    </div>

    <!-- Object list below treemap -->
    <div class="object-list">
      {#each (drillObject ? (root.children?.find((c) => c.label === drillObject)?.children ?? []) : (root.children ?? []))
        .slice()
        .sort((a, b) => b.totalBytes - a.totalBytes) as node (node.label)}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <div
          class="object-row"
          class:hovered={hovered === node.label}
          onmouseenter={() => (hovered = node.label)}
          onmouseleave={() => (hovered = null)}
          onclick={() => {
            if (!drillObject && node.children?.length) drillObject = node.label;
          }}
          onkeydown={(e) => {
            if ((e.key === "Enter" || e.key === " ") && !drillObject && node.children?.length)
              drillObject = node.label;
          }}
          role={node.children?.length && !drillObject ? "button" : "listitem"}
          tabindex={node.children?.length && !drillObject ? 0 : undefined}
          style="cursor: {!drillObject && node.children?.length ? 'pointer' : 'default'}"
        >
          <span class="obj-label">{node.label}</span>
          <span class="obj-count">{node.frameCount} frames</span>
          <div class="obj-bar-wrap">
            <div
              class="obj-bar"
              style="width: {root ? (node.totalBytes / root.totalBytes) * 100 : 0}%"
            ></div>
          </div>
          <span class="obj-size">{fmtBytes(node.totalBytes)}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .storage-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: auto;
    padding: 16px 20px;
    gap: 14px;
    min-height: 0;
  }

  .storage-header {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex-shrink: 0;
  }
  .storage-title-row {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  .storage-title {
    font-size: 1rem;
    font-weight: 700;
    color: var(--text-primary);
    margin: 0;
  }
  .storage-total {
    font-size: 0.8rem;
    color: var(--text-secondary);
  }

  .breadcrumb-btn {
    font-size: 0.78rem;
    padding: 3px 10px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    align-self: flex-start;
    transition:
      background 0.1s,
      color 0.1s;
  }
  .breadcrumb-btn:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }

  .storage-status {
    color: var(--text-secondary);
    font-size: 0.875rem;
    padding: 20px 0;
  }
  .storage-status.error {
    color: var(--danger);
  }

  /* ── Type summary bar ────────────────────────────────────────────────── */
  .type-bar {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    flex-shrink: 0;
  }
  .type-chip {
    display: flex;
    align-items: center;
    gap: 6px;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 4px 10px;
    font-size: 0.76rem;
  }
  .type-label {
    color: var(--text-secondary);
    text-transform: capitalize;
  }
  .type-size {
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  .type-count {
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }

  /* ── Treemap ─────────────────────────────────────────────────────────── */
  .treemap-container {
    position: relative;
    flex-shrink: 0;
  }
  .treemap-svg {
    display: block;
    border-radius: 6px;
    overflow: hidden;
    border: 1px solid var(--border);
  }

  :global(.tm-label) {
    font-size: 0.75rem;
    font-weight: 600;
    fill: rgba(255, 255, 255, 0.92);
    pointer-events: none;
  }
  :global(.tm-label-sm) {
    font-size: 0.62rem;
    fill: rgba(255, 255, 255, 0.75);
    pointer-events: none;
  }
  :global(.tm-sub) {
    font-size: 0.62rem;
    fill: rgba(255, 255, 255, 0.65);
    pointer-events: none;
  }

  .treemap-tooltip {
    position: absolute;
    bottom: 8px;
    left: 8px;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 6px 10px;
    font-size: 0.78rem;
    display: flex;
    flex-direction: column;
    gap: 2px;
    pointer-events: none;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
    z-index: 10;
    color: var(--text-primary);
  }
  .tooltip-hint {
    font-size: 0.68rem;
    color: var(--accent);
    margin-top: 2px;
  }

  /* ── Object list ─────────────────────────────────────────────────────── */
  .object-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex-shrink: 0;
  }

  .object-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 8px;
    border-radius: 4px;
    transition: background 0.1s;
  }
  .object-row.hovered {
    background: var(--bg-row-hover);
  }

  .obj-label {
    font-size: 0.82rem;
    color: var(--text-primary);
    min-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .obj-count {
    font-size: 0.72rem;
    color: var(--text-secondary);
    width: 70px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .obj-bar-wrap {
    flex: 1;
    height: 6px;
    background: var(--bg-base);
    border-radius: 3px;
    overflow: hidden;
  }
  .obj-bar {
    height: 100%;
    background: var(--accent);
    border-radius: 3px;
    transition: width 0.3s;
  }
  .obj-size {
    font-size: 0.78rem;
    color: var(--text-secondary);
    width: 70px;
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
</style>
