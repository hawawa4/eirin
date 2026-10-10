<script lang="ts">
  import type * as app from "$models/app";
  import {
    FIELD_ROTATION_NOTE_DEG,
    clusterName,
    formatDateRange,
    formatIntegration,
    neighbours,
    summarize,
  } from "../../lib/coverage/selection";
  import { isPlaced } from "../../lib/coverage/scene";
  import { plural } from "../../lib/utils";

  interface Props {
    /** Selected clusters, in selection order. */
    selected: app.CoverageCluster[];
    /** Every cluster (placed and unplaced), for neighbour suggestions. */
    all: app.CoverageCluster[];
    scopeLabel: (scope: string) => string;
    /** CSS colour of a scope's outline. */
    scopeCss: (scope: string) => string;
    onfocus: (c: app.CoverageCluster) => void;
    onadd: (c: app.CoverageCluster) => void;
    onremove: (c: app.CoverageCluster) => void;
    onclear: () => void;
    oncreateproject: () => void;
    onaddtoproject: () => void;
    onshowinlibrary: () => void;
  }

  let {
    selected,
    all,
    scopeLabel,
    scopeCss,
    onfocus,
    onadd,
    onremove,
    onclear,
    oncreateproject,
    onaddtoproject,
    onshowinlibrary,
  }: Props = $props();

  let sum = $derived(summarize(selected));
  let nearby = $derived(neighbours(all, selected));
</script>

<aside class="cov-panel" aria-label="Selected framings">
  <div class="cp-header">
    <div class="cp-title">
      {plural(selected.length, "framing")} selected
    </div>
    <button
      class="cp-close"
      onclick={onclear}
      title="Clear selection (Esc)"
      aria-label="Clear selection">✕</button
    >
  </div>

  <div class="cp-body">
    <dl class="cp-grid">
      <dt>Lights</dt>
      <dd>{sum.frames}</dd>
      <dt>Integration</dt>
      <dd>{formatIntegration(sum.expTotal)}</dd>
      {#if sum.firstDate}
        <dt>Captured</dt>
        <dd>{formatDateRange(sum.firstDate, sum.lastDate)}</dd>
      {/if}
      {#if sum.filters.length > 0}
        <dt>Filter</dt>
        <dd>{sum.filters.join(", ")}</dd>
      {/if}
      <dt>Scope</dt>
      <dd>{sum.scopes.map(scopeLabel).join(", ")}</dd>
    </dl>
    {#if sum.scopes.length > 1}
      <p class="cp-warn">
        Mixes telescopes: their pixel scales differ, so Siril must rescale to register them.
      </p>
    {/if}
    {#if sum.filters.length > 1}
      <p class="cp-warn">Mixes filters.</p>
    {/if}

    <div class="cp-actions">
      <button class="cp-primary" onclick={oncreateproject}>Create project…</button>
      <div class="cp-row">
        <button class="cp-secondary" onclick={onaddtoproject}>Add to project…</button>
        <button class="cp-secondary" onclick={onshowinlibrary}>Show in Library</button>
      </div>
    </div>

    <section class="cp-section">
      <div class="cp-section-head">Selection</div>
      <ul class="cp-list">
        {#each selected as c (c.id)}
          <li>
            <span class="cp-swatch" style:border-color={scopeCss(c.scope)} class:approx={c.approx}
            ></span>
            <button
              class="cp-item"
              onclick={() => onfocus(c)}
              disabled={!isPlaced(c)}
              title={isPlaced(c) ? "Show on the map" : "No position known"}
            >
              <span class="cp-item-name">{clusterName(c)}</span>
              <span class="cp-item-meta">
                {scopeLabel(c.scope)} · {plural(c.frames, "sub")} · {formatIntegration(c.expTotal)}
                {#if c.approx}· {isPlaced(c) ? "estimated" : "no position"}{/if}
                {#if c.rotationSpread >= FIELD_ROTATION_NOTE_DEG}
                  · rotates {Math.round(c.rotationSpread)}°
                {/if}
              </span>
            </button>
            <button
              class="cp-icon"
              onclick={() => onremove(c)}
              title="Remove from selection"
              aria-label="Remove {clusterName(c)} from selection">✕</button
            >
          </li>
        {/each}
      </ul>
    </section>

    {#if nearby.length > 0}
      <section class="cp-section">
        <div class="cp-section-head" title="Other framings overlapping the selection">
          Overlapping
        </div>
        <ul class="cp-list">
          {#each nearby as c (c.id)}
            <li>
              <span class="cp-swatch" style:border-color={scopeCss(c.scope)} class:approx={c.approx}
              ></span>
              <button class="cp-item" onclick={() => onfocus(c)} title="Show on the map">
                <span class="cp-item-name">{clusterName(c)}</span>
                <span class="cp-item-meta">
                  {scopeLabel(c.scope)} · {plural(c.frames, "sub")} · {formatIntegration(
                    c.expTotal,
                  )}
                </span>
              </button>
              <button
                class="cp-icon add"
                onclick={() => onadd(c)}
                title="Add to selection"
                aria-label="Add {clusterName(c)} to selection">+</button
              >
            </li>
          {/each}
        </ul>
      </section>
    {/if}
  </div>
</aside>

<style>
  .cov-panel {
    position: absolute;
    top: 48px;
    right: 10px;
    width: 290px;
    background: color-mix(in srgb, var(--bg-panel) 96%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 7px;
    z-index: 30;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    max-height: calc(100% - 120px);
    backdrop-filter: blur(4px);
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.4);
  }
  .cp-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 8px 8px 12px;
    border-bottom: 1px solid var(--border);
    gap: 6px;
    flex-shrink: 0;
  }
  .cp-title {
    font-size: var(--fs-md);
    font-weight: 600;
    color: var(--text-primary);
  }
  .cp-close,
  .cp-icon {
    flex-shrink: 0;
    width: 26px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: var(--fs-sm);
  }
  .cp-close:hover,
  .cp-icon:hover {
    color: var(--text-primary);
    border-color: var(--border);
    background: var(--bg-row-hover);
  }
  .cp-icon.add {
    font-size: var(--fs-lg);
    color: var(--accent);
  }

  .cp-body {
    padding: 10px 12px 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    overflow-y: auto;
    min-height: 0;
  }
  .cp-grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 3px 10px;
    margin: 0;
    font-size: var(--fs-sm);
  }
  .cp-grid dt {
    color: var(--text-secondary);
  }
  .cp-grid dd {
    margin: 0;
    color: var(--text-primary);
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .cp-warn {
    margin: 0;
    font-size: var(--fs-xs);
    color: var(--atlas-dso);
  }

  .cp-actions {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .cp-row {
    display: flex;
    gap: 6px;
  }
  .cp-primary,
  .cp-secondary {
    padding: 6px 0;
    border-radius: 5px;
    font-size: var(--fs-sm);
    font-weight: 600;
    cursor: pointer;
  }
  .cp-primary {
    background: var(--accent);
    border: 1px solid var(--accent);
    color: var(--accent-contrast);
  }
  .cp-primary:hover {
    filter: brightness(1.1);
  }
  .cp-secondary {
    flex: 1;
    background: var(--bg-row);
    border: 1px solid var(--border);
    color: var(--text-primary);
  }
  .cp-secondary:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .cp-section {
    border-top: 1px solid var(--border);
    padding-top: 8px;
  }
  .cp-section-head {
    font-size: var(--fs-xs);
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 4px;
  }
  .cp-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .cp-list li {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .cp-swatch {
    width: 12px;
    height: 12px;
    flex-shrink: 0;
    border: 2px solid;
    border-radius: 2px;
  }
  .cp-swatch.approx {
    border-style: dashed;
  }
  .cp-item {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    background: none;
    border: none;
    border-radius: 4px;
    padding: 3px 4px;
    cursor: pointer;
    text-align: left;
  }
  .cp-item:hover:not(:disabled) {
    background: var(--bg-row-hover);
  }
  .cp-item:disabled {
    cursor: default;
  }
  .cp-item-name {
    max-width: 100%;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .cp-item-meta {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
</style>
