<script lang="ts">
  import type { AnalysisProgress } from "../../lib/types";
  import type { LibGroup } from "../../lib/library/groups";

  interface Props {
    group: LibGroup;
    colspan: number;
    expanded: boolean;
    ontoggle: () => void;
    sirilAvailable: boolean;
    /** This group is being analyzed right now. */
    analyzing: boolean;
    /** Some group is being analyzed (disables the buttons everywhere). */
    analysisBusy: boolean;
    analysisProgress: AnalysisProgress | null;
    onanalyze: (force: boolean) => void;
    oncancelanalysis: () => void;
  }

  let {
    group,
    colspan,
    expanded,
    ontoggle,
    sirilAvailable,
    analyzing,
    analysisBusy,
    analysisProgress,
    onanalyze,
    oncancelanalysis,
  }: Props = $props();
</script>

<tr class="group-header-row" onclick={ontoggle}>
  <td {colspan}>
    <div class="group-hdr-inner">
      <button
        class="group-toggle"
        aria-expanded={expanded}
        onclick={(e) => {
          e.stopPropagation();
          ontoggle();
        }}
      >
        <span class="group-chevron" aria-hidden="true">{expanded ? "▼" : "▶"}</span>
        <span class="group-label">{group.label}</span>
      </button>
      <span class="group-count"
        >{group.frames.length} frame{group.frames.length !== 1 ? "s" : ""}</span
      >
      <span class="group-type-breakdown">
        {#each group.breakdown as { type, count, meta } (type)}
          <span class="group-type-badge" style="color:{meta.color};background:{meta.bg}">
            {meta.short}
            {count}
          </span>
        {/each}
      </span>
      {#if group.hasQuality}
        <span class="quality-dot" title="Quality data available">✦</span>
      {/if}
      <span class="group-spacer"></span>
      {#if sirilAvailable && group.frames.length > 0}
        {#if analyzing}
          <span class="analysis-status">
            ⟳ {analysisProgress?.done ?? 0}/{analysisProgress?.total ?? group.unanalyzedCount}
            {#if analysisProgress?.current}· {analysisProgress.current}{/if}
          </span>
          <button
            class="btn-cancel-analysis"
            onclick={(e) => {
              e.stopPropagation();
              oncancelanalysis();
            }}
            title="Cancel analysis">Cancel</button
          >
        {:else}
          {#if group.unanalyzedCount > 0}
            <button
              class="btn-analyze"
              onclick={(e) => {
                e.stopPropagation();
                onanalyze(false);
              }}
              disabled={analysisBusy}
              title="Analyze unanalyzed frames with Siril (findstar + platesolve)">✦ Analyze</button
            >
          {/if}
          <button
            class="btn-analyze btn-reanalyze"
            onclick={(e) => {
              e.stopPropagation();
              onanalyze(true);
            }}
            disabled={analysisBusy}
            title="Re-run analysis on all frames, including already-analyzed ones"
            >↺ Reanalyze</button
          >
        {/if}
      {/if}
    </div>
  </td>
</tr>

<style>
  .group-header-row {
    cursor: pointer;
    user-select: none;
  }

  .group-header-row:hover td {
    background: color-mix(in srgb, var(--bg-panel) 70%, var(--accent) 30%);
  }

  .group-header-row td {
    background: color-mix(in srgb, var(--bg-panel) 85%, var(--accent) 15%);
    border-bottom: 1px solid var(--border-accent);
    padding: 4px 10px 4px 4px;
  }

  .group-hdr-inner {
    display: flex;
    align-items: center;
    /* Fixed so every header has the same pitch (the table body is virtualized). */
    height: 1.625rem;
    overflow: hidden;
    gap: 8px;
    width: 100%;
    min-width: 0;
  }

  .group-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    background: transparent;
    border: none;
    color: inherit;
    padding: 2px 4px;
    cursor: pointer;
    min-width: 0;
    border-radius: 3px;
  }

  .group-chevron {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    display: inline-block;
    width: 12px;
  }

  .group-label {
    font-size: var(--fs-md);
    font-weight: 600;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .group-count {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .group-type-breakdown {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    flex-wrap: nowrap;
  }

  .group-type-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: var(--fs-xs);
    font-weight: 700;
    font-family: monospace;
    padding: 1px 6px;
    border-radius: 3px;
    letter-spacing: 0.04em;
    white-space: nowrap;
  }

  .group-spacer {
    flex: 1;
  }

  .quality-dot {
    font-size: var(--fs-xs);
    color: var(--accent);
  }

  .btn-analyze {
    font-size: var(--fs-xs);
    padding: 2px 8px;
    background: transparent;
    border: 1px solid var(--accent);
    color: var(--accent);
    border-radius: 3px;
    cursor: pointer;
    transition:
      background 0.12s,
      color 0.12s;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .btn-analyze:hover:not(:disabled) {
    background: var(--accent);
    color: var(--accent-contrast);
  }

  .btn-analyze:disabled {
    border-color: var(--border);
    color: var(--text-dim);
    cursor: default;
  }

  .btn-reanalyze {
    border-color: var(--border-accent);
    color: var(--text-secondary);
  }
  .btn-reanalyze:hover:not(:disabled) {
    background: color-mix(in srgb, var(--accent) 20%, transparent);
    color: var(--text-primary);
    border-color: var(--accent);
  }

  .analysis-status {
    font-size: var(--fs-xs);
    color: var(--accent);
    white-space: nowrap;
    flex-shrink: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .btn-cancel-analysis {
    font-size: var(--fs-xs);
    padding: 2px 8px;
    background: transparent;
    border: 1px solid var(--text-secondary);
    color: var(--text-secondary);
    border-radius: 3px;
    cursor: pointer;
    flex-shrink: 0;
  }

  .btn-cancel-analysis:hover {
    border-color: var(--danger);
    color: var(--danger);
  }
</style>
