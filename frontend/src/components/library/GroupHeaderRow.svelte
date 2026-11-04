<script lang="ts">
  import type { AnalysisProgress } from "../../lib/types";
  import type { LibGroup } from "../../lib/library/groups";

  interface Props {
    group: LibGroup;
    colspan: number;
    expanded: boolean;
    ontoggle: () => void;
    /** How many of the group's frames are checked. */
    checkState: "none" | "some" | "all";
    /** Checks every frame of the group, or unchecks them when all are checked. */
    oncheck: () => void;
    /** Opens the group's action menu at (x, y). */
    onmenu: (x: number, y: number) => void;
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
    checkState,
    oncheck,
    onmenu,
    sirilAvailable,
    analyzing,
    analysisBusy,
    analysisProgress,
    onanalyze,
    oncancelanalysis,
  }: Props = $props();

  let analyzedCount = $derived(group.frames.length - group.unanalyzedCount);
</script>

<tr
  class="group-header-row"
  onclick={ontoggle}
  oncontextmenu={(e) => {
    e.preventDefault();
    onmenu(e.clientX, e.clientY);
  }}
>
  <td {colspan}>
    <div class="group-hdr-inner">
      <input
        type="checkbox"
        class="group-check"
        checked={checkState === "all"}
        {@attach (el: HTMLInputElement) => {
          el.indeterminate = checkState === "some";
        }}
        aria-label="Select all {group.frames.length} frames in {group.label}"
        title={checkState === "all" ? "Unselect this group" : "Select every frame in this group"}
        onclick={(e) => {
          e.stopPropagation();
          oncheck();
        }}
      />
      <button
        class="group-toggle"
        aria-expanded={expanded}
        onclick={(e) => {
          e.stopPropagation();
          ontoggle();
        }}
      >
        <span class="group-chevron" aria-hidden="true">{expanded ? "▾" : "▸"}</span>
        <span class="group-label">{group.label}</span>
      </button>
      <span class="group-count"
        >{group.frames.length.toLocaleString()} frame{group.frames.length !== 1 ? "s" : ""}</span
      >
      <span class="group-type-breakdown">
        {#each group.breakdown as { type, count, meta } (type)}
          <span
            class="group-type-badge"
            style="color:{meta.color};background:{meta.bg}"
            title="{count} {meta.label.toLowerCase()} frame{count !== 1 ? 's' : ''}"
          >
            {meta.short}
            {count}
          </span>
        {/each}
      </span>
      {#if group.hasQuality}
        <span
          class="quality-mark"
          class:partial={group.unanalyzedCount > 0}
          title={group.unanalyzedCount > 0
            ? `${analyzedCount} of ${group.frames.length} frames analyzed (FWHM, stars, plate solve)`
            : "Every frame analyzed (FWHM, stars, plate solve)"}
          aria-label={group.unanalyzedCount > 0 ? "Partly analyzed" : "Analyzed"}
          >{group.unanalyzedCount > 0 ? "◐" : "✓"}</span
        >
      {/if}
      <span class="group-spacer"></span>
      {#if analyzing}
        <span class="analysis-status">
          Analyzing {analysisProgress?.done ?? 0}/{analysisProgress?.total ?? group.unanalyzedCount}
          {#if analysisProgress?.current}· {analysisProgress.current}{/if}
        </span>
        <button
          class="hdr-btn cancel"
          onclick={(e) => {
            e.stopPropagation();
            oncancelanalysis();
          }}
          title="Cancel analysis">Cancel</button
        >
      {:else}
        <span class="hover-actions">
          {#if sirilAvailable && group.unanalyzedCount > 0}
            <button
              class="hdr-btn accent"
              onclick={(e) => {
                e.stopPropagation();
                onanalyze(false);
              }}
              disabled={analysisBusy}
              title="Measure FWHM, stars and background and plate-solve the {group.unanalyzedCount} unanalyzed frames with Siril"
              >Analyze {group.unanalyzedCount.toLocaleString()}</button
            >
          {/if}
          <button
            class="hdr-btn icon"
            aria-label="Actions for {group.label}"
            title="Actions for this group (or right-click)"
            onclick={(e) => {
              e.stopPropagation();
              const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
              onmenu(r.right - 200, r.bottom + 2);
            }}>⋯</button
          >
        </span>
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
    padding: 4px 10px 4px 0;
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

  /* Centred in the 28px checkbox column, like the frame rows' boxes. */
  .group-check {
    width: 28px;
    flex-shrink: 0;
    margin: 0 -6px 0 0;
    accent-color: var(--accent);
    cursor: pointer;
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
    font-size: var(--fs-sm);
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

  .quality-mark {
    font-size: var(--fs-xs);
    color: var(--success);
    cursor: help;
  }
  .quality-mark.partial {
    color: var(--text-secondary);
  }

  /* Group actions appear on hover / keyboard focus instead of repeating on every row. */
  .hover-actions {
    display: inline-flex;
    gap: 6px;
    opacity: 0;
    transition: opacity 0.1s;
  }
  .group-header-row:hover .hover-actions,
  .hover-actions:focus-within {
    opacity: 1;
  }

  .hdr-btn {
    font-size: var(--fs-xs);
    padding: 2px 8px;
    background: var(--bg-base);
    border: 1px solid var(--border-accent);
    color: var(--text-secondary);
    border-radius: 4px;
    cursor: pointer;
    white-space: nowrap;
    flex-shrink: 0;
  }
  .hdr-btn:hover:not(:disabled) {
    color: var(--text-primary);
    border-color: var(--accent);
  }
  .hdr-btn:disabled {
    color: var(--text-dim);
    cursor: default;
  }
  .hdr-btn.accent {
    border-color: var(--accent);
    color: var(--accent);
  }
  .hdr-btn.accent:hover:not(:disabled) {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .hdr-btn.icon {
    width: 26px;
    padding: 2px 0;
    font-size: var(--fs-sm);
    line-height: 1;
  }
  .hdr-btn.cancel:hover {
    border-color: var(--danger);
    color: var(--danger);
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
</style>
