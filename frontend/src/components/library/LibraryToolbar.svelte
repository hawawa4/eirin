<script lang="ts">
  import type { ColumnDef, FrameType, LibraryGroupBy } from "../../lib/types";
  import { GROUP_BY_OPTIONS } from "../../lib/library/groups";
  import TypeChips from "./TypeChips.svelte";

  interface Props {
    showRejected: boolean;
    rejectedCount: number;
    ontab: (rejected: boolean) => void;
    groupBy: LibraryGroupBy;
    ongroupby: (g: LibraryGroupBy) => void;
    /** Raw search box text (debounced by the parent). */
    search: string;
    onexpandall: () => void;
    oncollapseall: () => void;
    /** Label of the active sort column, or null. */
    sortLabel: string | null;
    onclearsort: () => void;
    filtersActive: boolean;
    onclearfilters: () => void;
    refreshing: boolean;
    /** Frames shown / frames in this tab. */
    visibleCount: number;
    tabCount: number;
    /** Type chips: per-type counts that pass the other filters. */
    typeCounts: { type: string; count: number }[];
    typeTotal: number;
    selectedTypes: readonly FrameType[];
    onpicktype: (type: FrameType | null, additive: boolean) => void;
    /** Shown only when provided (Siril available, Frames tab). */
    onsuggest?: () => void;
    /** Shown only when provided (desktop mode). Creates a project from the filtered view. */
    oncreateproject?: () => void;
    /** Lights, darks, flats and biases in the current view. */
    projectFrameCount: number;
    columns: ColumnDef[];
    ontogglecolumn: (colId: string) => void;
    /** The per-column filter row under the table header. */
    filterRowOpen: boolean;
    /** Column filters set (other than type), shown as a badge. */
    colFilterCount: number;
    ontogglefilterrow: () => void;
    shortcutsOpen: boolean;
    onshortcuts: () => void;
  }

  let {
    showRejected,
    rejectedCount,
    ontab,
    groupBy,
    ongroupby,
    search = $bindable(),
    onexpandall,
    oncollapseall,
    sortLabel,
    onclearsort,
    filtersActive,
    onclearfilters,
    refreshing,
    visibleCount,
    tabCount,
    typeCounts,
    typeTotal,
    selectedTypes,
    onpicktype,
    onsuggest,
    oncreateproject,
    projectFrameCount,
    columns,
    ontogglecolumn,
    filterRowOpen,
    colFilterCount,
    ontogglefilterrow,
    shortcutsOpen,
    onshortcuts,
  }: Props = $props();

  let searchEl = $state<HTMLInputElement | null>(null);

  /** Focuses the search box (the `/` shortcut). */
  export function focusSearch() {
    searchEl?.focus();
    searchEl?.select();
  }

  let showColumnMenu = $state(false);
  let colMenuRoot = $state<HTMLDivElement | null>(null);

  // Close column menu on outside click / Escape
  $effect(() => {
    if (!showColumnMenu) return;
    function onDoc(e: MouseEvent) {
      if (colMenuRoot && !colMenuRoot.contains(e.target as Node)) showColumnMenu = false;
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.stopPropagation();
        showColumnMenu = false;
      }
    }
    document.addEventListener("mousedown", onDoc);
    window.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      window.removeEventListener("keydown", onKey, true);
    };
  });

  let sortedColumns = $derived(
    [...columns]
      .filter((c) => c.id !== "frameType" && c.id !== "name")
      .sort((a, b) => a.order - b.order),
  );
</script>

<div class="toolbar">
  <div class="row">
    <div class="tabs" role="tablist" aria-label="Library view">
      <button
        class="tab"
        role="tab"
        aria-selected={!showRejected}
        class:active={!showRejected}
        onclick={() => ontab(false)}
        title="Frames you're keeping">Frames</button
      >
      <button
        class="tab"
        role="tab"
        aria-selected={showRejected}
        class:active={showRejected}
        onclick={() => ontab(true)}
        title="Frames you rejected (x). They stay on disk until you delete them."
      >
        Rejected
        {#if rejectedCount > 0}<span class="tab-badge">{rejectedCount.toLocaleString()}</span>{/if}
      </button>
    </div>

    <div class="search-wrap">
      <span class="search-icon" aria-hidden="true">⌕</span>
      <input
        bind:this={searchEl}
        class="search-input"
        type="search"
        placeholder="Search name, object, filter…"
        aria-label="Search frames"
        title="Search file names, objects and filters ( / )"
        spellcheck="false"
        bind:value={search}
        onkeydown={(e) => {
          if (e.key === "Escape" && search) {
            e.stopPropagation();
            search = "";
          } else if (e.key === "Escape" || e.key === "Enter") {
            (e.currentTarget as HTMLInputElement).blur();
          }
        }}
      />
      <kbd class="search-kbd" aria-hidden="true">/</kbd>
    </div>

    <label class="group-by" title="Group rows by">
      <span class="tb-label">Group by</span>
      <select
        value={groupBy}
        onchange={(e) => ongroupby((e.currentTarget as HTMLSelectElement).value as LibraryGroupBy)}
      >
        {#each GROUP_BY_OPTIONS as opt (opt.value)}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
    </label>
    <div class="icon-pair">
      <button
        class="icon-btn"
        onclick={onexpandall}
        title="Expand all groups"
        aria-label="Expand all groups">⊞</button
      >
      <button
        class="icon-btn"
        onclick={oncollapseall}
        title="Collapse all groups"
        aria-label="Collapse all groups">⊟</button
      >
    </div>

    <span class="spacer"></span>

    {#if refreshing}
      <span class="refreshing" role="status"><span class="mini-spinner"></span>Refreshing…</span>
    {/if}

    {#if onsuggest && !showRejected}
      <button
        class="tool-btn"
        onclick={onsuggest}
        title="Find frames with outlying FWHM, star count or background and offer to reject them (needs analyzed frames)"
      >
        Suggest rejects…
      </button>
    {/if}
    {#if oncreateproject && !showRejected}
      <button
        class="tool-btn primary"
        onclick={oncreateproject}
        disabled={projectFrameCount === 0}
        title={projectFrameCount === 0
          ? "No light, dark, flat or bias frames in this view"
          : `New Siril project from the ${projectFrameCount.toLocaleString()} light, dark, flat and bias frames shown (stacks and processed images are left out). Narrow the view first with the type chips and search.`}
      >
        ＋ New project
        {#if projectFrameCount > 0}<span class="btn-count"
            >{projectFrameCount.toLocaleString()}</span
          >{/if}
      </button>
    {/if}

    <span class="divider" aria-hidden="true"></span>

    <button
      class="icon-btn"
      class:active={filterRowOpen}
      aria-pressed={filterRowOpen}
      onclick={ontogglefilterrow}
      title="Column filters: filter by any column, e.g. FWHM < 3 or a telescope"
      aria-label="Column filters"
    >
      <span aria-hidden="true">⫶</span>
      {#if colFilterCount > 0}<span class="icon-badge">{colFilterCount}</span>{/if}
    </button>
    <div class="column-selector" bind:this={colMenuRoot}>
      <button
        class="icon-btn"
        class:active={showColumnMenu}
        aria-expanded={showColumnMenu}
        onclick={() => (showColumnMenu = !showColumnMenu)}
        title="Show or hide columns"
        aria-label="Columns">▥</button
      >
      {#if showColumnMenu}
        <div class="column-menu">
          <span class="column-menu-title">Columns</span>
          {#each sortedColumns as col (col.id)}
            <label class="column-menu-item">
              <input
                type="checkbox"
                checked={col.visible}
                onchange={() => ontogglecolumn(col.id)}
              />
              {col.label}
            </label>
          {/each}
          <p class="column-menu-hint">Drag headers to reorder, drag their edges to resize.</p>
        </div>
      {/if}
    </div>
    <button
      class="icon-btn"
      data-shortcuts-toggle
      class:active={shortcutsOpen}
      aria-expanded={shortcutsOpen}
      onclick={onshortcuts}
      title="Keyboard shortcuts (?)"
      aria-label="Keyboard shortcuts">?</button
    >
  </div>

  <div class="row row-chips">
    <TypeChips counts={typeCounts} total={typeTotal} selected={selectedTypes} onpick={onpicktype} />
    <span class="spacer"></span>
    <span class="showing" aria-live="polite">
      {#if visibleCount === tabCount}
        {tabCount.toLocaleString()} {tabCount === 1 ? "frame" : "frames"}
      {:else}
        Showing <strong>{visibleCount.toLocaleString()}</strong> of {tabCount.toLocaleString()}
      {/if}
    </span>
    {#if sortLabel}
      <button class="clear-chip" onclick={onclearsort} title="Back to the default order"
        >Sorted by {sortLabel} <span aria-hidden="true">✕</span></button
      >
    {/if}
    {#if filtersActive}
      <button
        class="clear-chip"
        onclick={onclearfilters}
        title="Clear search, type and column filters"
        >Clear filters <span aria-hidden="true">✕</span></button
      >
    {/if}
  </div>
</div>

<style>
  .toolbar {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 6px 10px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 8px;
    min-width: 0;
  }

  .spacer {
    flex: 1;
  }

  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
    margin: 0 2px;
  }

  /* ── Tabs ── */
  .tabs {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 5px;
    overflow: hidden;
    flex-shrink: 0;
  }
  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 12px;
    font-size: var(--fs-sm);
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
  }
  .tab + .tab {
    border-left: 1px solid var(--border);
  }
  .tab:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .tab.active,
  .tab.active:hover {
    background: var(--accent);
    color: var(--accent-contrast);
    font-weight: 600;
  }
  .tab-badge {
    background: var(--danger);
    color: #fff;
    border-radius: 8px;
    padding: 0 6px;
    font-size: var(--fs-xs);
    font-weight: 600;
    line-height: 1.4;
  }

  /* ── Search ── */
  .search-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }
  .search-icon {
    position: absolute;
    left: 8px;
    color: var(--text-dim);
    font-size: var(--fs-md);
    pointer-events: none;
  }
  .search-input {
    font-size: var(--fs-sm);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 5px;
    color: var(--text-primary);
    padding: 4px 26px 4px 26px;
    width: 260px;
    min-width: 140px;
    outline: none;
  }
  .search-input::placeholder {
    color: var(--text-dim);
  }
  .search-input:focus {
    border-color: var(--accent);
  }
  .search-kbd {
    position: absolute;
    right: 6px;
    font-family: inherit;
    font-size: var(--fs-xs);
    color: var(--text-dim);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 0 5px;
    pointer-events: none;
  }
  .search-input:focus + .search-kbd,
  .search-input:not(:placeholder-shown) + .search-kbd {
    display: none;
  }

  /* ── Group by ── */
  .group-by {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .tb-label {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .group-by select {
    font-size: var(--fs-sm);
    background: var(--bg-base);
    color: var(--text-primary);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 3px 6px;
    cursor: pointer;
  }
  .group-by select:hover {
    border-color: var(--accent);
  }

  .icon-pair {
    display: flex;
    gap: 2px;
  }

  /* ── Icon buttons (with tooltips) ── */
  .icon-btn {
    position: relative;
    width: 28px;
    height: 26px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 5px;
    color: var(--text-secondary);
    font-size: var(--fs-md);
    cursor: pointer;
    flex-shrink: 0;
  }
  .icon-btn:hover {
    color: var(--accent);
    border-color: var(--accent);
  }
  .icon-btn.active,
  .icon-btn.active:hover {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }
  .icon-badge {
    position: absolute;
    top: -6px;
    right: -6px;
    min-width: 15px;
    height: 15px;
    padding: 0 3px;
    border-radius: 8px;
    background: var(--accent);
    color: var(--accent-contrast);
    font-size: 0.625rem;
    font-weight: 700;
    line-height: 15px;
    text-align: center;
  }
  .icon-btn.active .icon-badge {
    background: var(--text-primary);
  }

  .tool-btn {
    font-size: var(--fs-sm);
    padding: 3px 10px;
    min-height: 26px;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .tool-btn:disabled {
    color: var(--text-dim);
    border-color: var(--border);
    cursor: default;
  }
  .tool-btn.primary:not(:disabled) {
    color: var(--accent);
    border-color: var(--accent);
  }
  .tool-btn.primary:hover:not(:disabled) {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .btn-count {
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
    opacity: 0.8;
  }

  .refreshing {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .mini-spinner {
    width: 12px;
    height: 12px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* ── Chips row ── */
  .showing {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .showing strong {
    color: var(--text-primary);
  }
  .clear-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: var(--fs-xs);
    padding: 2px 9px;
    min-height: 24px;
    background: transparent;
    border: 1px solid var(--accent);
    border-radius: 12px;
    color: var(--accent);
    cursor: pointer;
    white-space: nowrap;
  }
  .clear-chip:hover {
    background: color-mix(in srgb, var(--accent) 18%, transparent);
  }

  /* ── Column menu ── */
  .column-selector {
    position: relative;
    flex-shrink: 0;
  }
  .column-menu {
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 5px 0;
    z-index: 200;
    min-width: 190px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.45);
  }
  .column-menu-title {
    display: block;
    padding: 3px 12px 4px;
    font-size: var(--fs-xs);
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .column-menu-item {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 4px 12px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
  }
  .column-menu-item:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .column-menu-item input {
    accent-color: var(--accent);
    cursor: pointer;
  }
  .column-menu-hint {
    margin: 4px 12px 2px;
    padding-top: 6px;
    border-top: 1px solid var(--border);
    font-size: var(--fs-xs);
    color: var(--text-dim);
    max-width: 200px;
  }
</style>
