<script lang="ts">
  import type * as app from "$models/app";
  import {
    FIELD_ROTATION_NOTE_DEG,
    clusterName,
    formatDateRange,
    formatIntegration,
  } from "../../lib/coverage/selection";
  import { plural } from "../../lib/utils";

  interface Props {
    cluster: app.CoverageCluster;
    scopeLabel: string;
    x: number;
    y: number;
    boundsW: number;
    boundsH: number;
    selected: boolean;
  }

  let { cluster, scopeLabel, x, y, boundsW, boundsH, selected }: Props = $props();

  const W = 260;
  const H = 130;
  let left = $derived(x + 16 + W > boundsW ? Math.max(4, x - 16 - W) : x + 16);
  let top = $derived(Math.max(4, Math.min(y - 10, boundsH - H)));
</script>

<div class="cov-tooltip" style="left: {left}px; top: {top}px; width: {W}px;">
  <div class="tt-name">{clusterName(cluster)}</div>
  <div class="tt-row">
    <span>{scopeLabel}</span>
    <span>{plural(cluster.frames, "sub")}</span>
    <span>{formatIntegration(cluster.expTotal)}</span>
  </div>
  <div class="tt-row">
    <span>{formatDateRange(cluster.firstDate, cluster.lastDate)}</span>
    {#if cluster.nights > 1}<span>{cluster.nights} nights</span>{/if}
  </div>
  {#if cluster.filters.length > 0 || cluster.rotationSpread >= FIELD_ROTATION_NOTE_DEG}
    <div class="tt-row">
      {#if cluster.filters.length > 0}<span>Filter {cluster.filters.join(", ")}</span>{/if}
      {#if cluster.rotationSpread >= FIELD_ROTATION_NOTE_DEG}
        <span>Field rotation {Math.round(cluster.rotationSpread)}°</span>
      {/if}
    </div>
  {/if}
  {#if cluster.approx}
    <div class="tt-approx">Position estimated from the object name</div>
  {/if}
  <div class="tt-hint">
    {selected ? "Shift/Ctrl+click to deselect" : "Click to select · Shift/Ctrl+click to add"}
  </div>
</div>

<style>
  .cov-tooltip {
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
  .tt-approx {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-style: italic;
  }
  .tt-hint {
    margin-top: 3px;
    font-size: var(--fs-xs);
    color: var(--accent);
  }
</style>
