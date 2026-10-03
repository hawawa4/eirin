<script lang="ts">
  import type * as app from "$models/app";
  import { formatDec, formatRA } from "../../lib/utils";

  interface Props {
    entry: app.AtlasIndexEntry;
    x: number;
    y: number;
    boundsW: number;
    boundsH: number;
    overlaid: boolean;
    selected: boolean;
  }

  let { entry, x, y, boundsW, boundsH, overlaid, selected }: Props = $props();

  const W = 240;
  const H = 110;
  let left = $derived(x + 16 + W > boundsW ? Math.max(4, x - 16 - W) : x + 16);
  let top = $derived(Math.max(4, Math.min(y - 10, boundsH - H)));
</script>

<div class="atlas-tooltip" style="left: {left}px; top: {top}px; width: {W}px;">
  <div class="tt-name">{entry.object || entry.name}</div>
  <div class="tt-row">
    <span>{formatRA(entry.ra)}</span>
    <span>{formatDec(entry.dec)}</span>
  </div>
  <div class="tt-row">{entry.pixelScale.toFixed(2)} ″/px · {entry.frameType}</div>
  <div class="tt-hint">
    {#if selected}
      Shift/Ctrl+click to remove from overlay
    {:else if overlaid}
      Click to show only this · Shift/Ctrl+click to remove
    {:else}
      Click to preview · Shift/Ctrl+click to add to overlay
    {/if}
  </div>
</div>

<style>
  .atlas-tooltip {
    position: absolute;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 6px;
    padding: 7px 10px;
    pointer-events: none;
    z-index: 25;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .tt-name {
    font-size: var(--fs-sm);
    font-weight: 600;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .tt-row {
    display: flex;
    gap: 10px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
  .tt-hint {
    margin-top: 3px;
    font-size: var(--fs-xs);
    color: var(--accent);
  }
</style>
