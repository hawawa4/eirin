<script lang="ts">
  import type { StorageGroup } from "../../lib/storage/tree";
  import { squarify } from "../../lib/storage/treemap";
  import { fitLabel, fmtFrames } from "../../lib/storage/format";
  import { formatBytes } from "../../lib/utils";

  interface Props {
    nodes: StorageGroup[];
    /** Label of the hovered node (shared with the list below the map). */
    hovered: string | null;
    onhover: (label: string | null) => void;
    /** Called for nodes that have children. */
    ondrill: (node: StorageGroup) => void;
    /** Right-click / context-menu key on a tile. */
    onmenu: (node: StorageGroup, x: number, y: number) => void;
  }
  let { nodes, hovered, onhover, ondrill, onmenu }: Props = $props();

  let container: HTMLDivElement;
  let width = $state(0);
  let remPx = $state(16);

  // Follow the container's width; the height follows the width within sane bounds.
  $effect(() => {
    const ro = new ResizeObserver((entries) => {
      width = Math.floor(entries[0]?.contentRect.width ?? 0);
      remPx = parseFloat(getComputedStyle(document.documentElement).fontSize) || 16;
    });
    ro.observe(container);
    return () => ro.disconnect();
  });

  let height = $derived(Math.round(Math.min(460, Math.max(220, width * 0.5))));
  let labelPx = $derived(remPx * 0.8125); // --fs-sm
  let subPx = $derived(remPx * 0.75); // --fs-xs

  let maxBytes = $derived(nodes.reduce((m, n) => Math.max(m, n.totalBytes), 0));

  let cells = $derived.by(() => {
    if (width <= 0) return [];
    return squarify(
      nodes.map((n) => ({ value: n.totalBytes, data: n })),
      0,
      0,
      width,
      height,
    ).map((r) => {
      const n = r.data;
      const pad = 6;
      const innerW = r.w - pad * 2;
      const lineH = labelPx * 1.35;
      const subH = subPx * 1.35;
      const label = r.h >= lineH + pad ? fitLabel(n.label, innerW, labelPx) : null;
      const size =
        label && r.h >= lineH + subH + pad
          ? fitLabel(formatBytes(n.totalBytes), innerW, subPx)
          : null;
      const frames =
        size && r.h >= lineH + subH * 2 + pad
          ? fitLabel(fmtFrames(n.frameCount), innerW, subPx)
          : null;
      const lines = [label, size, frames].filter((l) => l !== null).length;
      const blockH = lines === 0 ? 0 : lineH + (lines - 1) * subH;
      // Darker (stronger) fill for larger nodes: magnitude on a single hue.
      const strength = maxBytes > 0 ? Math.sqrt(n.totalBytes / maxBytes) : 1;
      return {
        ...r,
        node: n,
        label,
        size,
        frames,
        top: r.y + (r.h - blockH) / 2,
        lineH,
        subH,
        fill: `color-mix(in srgb, var(--accent-dim) ${Math.round(40 + 60 * strength)}%, var(--bg-panel))`,
        drillable: !!n.children?.length,
        rejectedFrac: n.totalBytes > 0 ? n.rejectedBytes / n.totalBytes : 0,
      };
    });
  });

  let pointer = $state<{ x: number; y: number } | null>(null);
  let hoveredNode = $derived(nodes.find((n) => n.label === hovered) ?? null);

  function onMove(e: MouseEvent) {
    const r = container.getBoundingClientRect();
    pointer = { x: e.clientX - r.left, y: e.clientY - r.top };
  }
</script>

<div
  class="treemap"
  bind:this={container}
  onmousemove={onMove}
  onmouseleave={() => {
    pointer = null;
    onhover(null);
  }}
  role="presentation"
>
  {#if width > 0}
    <svg {width} {height} viewBox="0 0 {width} {height}" aria-label="Storage treemap">
      {#each cells as c (c.node.label)}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
        <g
          class="cell"
          class:drillable={c.drillable}
          class:hovered={hovered === c.node.label}
          role={c.drillable ? "button" : "img"}
          tabindex={c.drillable ? 0 : undefined}
          aria-label="{c.node.label}: {formatBytes(c.node.totalBytes)}, {fmtFrames(
            c.node.frameCount,
          )}"
          onmouseenter={() => onhover(c.node.label)}
          onfocus={() => onhover(c.node.label)}
          onblur={() => onhover(null)}
          onclick={() => c.drillable && ondrill(c.node)}
          oncontextmenu={(e) => {
            e.preventDefault();
            onmenu(c.node, e.clientX, e.clientY);
          }}
          onkeydown={(e) => {
            if (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) {
              e.preventDefault();
              const r = (e.currentTarget as SVGGElement).getBoundingClientRect();
              onmenu(c.node, r.left + r.width / 2, r.top + r.height / 2);
              return;
            }
            if (c.drillable && (e.key === "Enter" || e.key === " ")) {
              e.preventDefault();
              ondrill(c.node);
            }
          }}
        >
          <rect
            x={c.x + 1}
            y={c.y + 1}
            width={Math.max(0, c.w - 2)}
            height={Math.max(0, c.h - 2)}
            rx="4"
            style:fill={c.fill}
          />
          {#if c.rejectedFrac > 0 && c.w > 12 && c.h > 12}
            <!-- Share of the tile taken by rejected frames (space you can free). -->
            <rect
              class="tm-rejected"
              x={c.x + 3}
              y={c.y + c.h - 6}
              width={Math.max(2, (c.w - 6) * c.rejectedFrac)}
              height="3"
              rx="1.5"
            />
          {/if}
          {#if c.label}
            <text x={c.x + c.w / 2} y={c.top + c.lineH / 2} class="tm-label">{c.label}</text>
          {/if}
          {#if c.size}
            <text x={c.x + c.w / 2} y={c.top + c.lineH + c.subH / 2} class="tm-sub">{c.size}</text>
          {/if}
          {#if c.frames}
            <text x={c.x + c.w / 2} y={c.top + c.lineH + c.subH * 1.5} class="tm-sub"
              >{c.frames}</text
            >
          {/if}
        </g>
      {/each}
    </svg>
  {/if}

  {#if hoveredNode && pointer}
    <div
      class="tooltip"
      style:left="{Math.min(pointer.x + 14, Math.max(0, width - 220))}px"
      style:top="{Math.min(pointer.y + 14, Math.max(0, height - 90))}px"
    >
      <strong>{hoveredNode.label}</strong>
      <span>{formatBytes(hoveredNode.totalBytes)} · {fmtFrames(hoveredNode.frameCount)}</span>
      {#if hoveredNode.rejectedBytes > 0}
        <span class="tooltip-rejected"
          >{formatBytes(hoveredNode.rejectedBytes)} in {fmtFrames(hoveredNode.rejectedPaths.length)} rejected</span
        >
      {/if}
      <span class="tooltip-hint"
        >{hoveredNode.children?.length ? "Click to open · " : ""}Right-click for actions</span
      >
    </div>
  {/if}
</div>

<style>
  .treemap {
    position: relative;
    width: 100%;
    flex-shrink: 0;
  }
  svg {
    display: block;
    border-radius: 6px;
    background: var(--bg-base);
  }
  .cell rect {
    stroke: transparent;
    stroke-width: 2;
    transition: stroke 0.1s;
  }
  .cell.drillable {
    cursor: pointer;
  }
  .cell.hovered rect,
  .cell:focus-visible rect {
    stroke: var(--accent);
  }
  .cell:focus {
    outline: none;
  }
  .tm-label,
  .tm-sub {
    text-anchor: middle;
    dominant-baseline: central;
    pointer-events: none;
  }
  .tm-label {
    font-size: var(--fs-sm);
    font-weight: 600;
    fill: var(--text-primary);
  }
  .tm-sub {
    font-size: var(--fs-xs);
    /* --text-secondary is too low-contrast on the accent fills */
    fill: var(--text-primary);
  }

  .tooltip {
    position: absolute;
    max-width: 220px;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 6px 10px;
    font-size: var(--fs-sm);
    display: flex;
    flex-direction: column;
    gap: 2px;
    pointer-events: none;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
    z-index: 10;
    color: var(--text-primary);
    word-break: break-word;
  }
  .tm-rejected {
    fill: var(--danger);
    opacity: 0.85;
    pointer-events: none;
  }
  .tooltip-rejected {
    font-size: var(--fs-xs);
    color: var(--danger);
  }
  .tooltip-hint {
    font-size: var(--fs-xs);
    color: var(--accent);
  }
</style>
