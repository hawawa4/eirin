<script lang="ts">
  import type { ColumnDef, LibraryGroupBy } from "../../lib/types";
  import { GROUP_BY_OPTIONS } from "../../lib/library/groups";

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
    sortActive: boolean;
    onclearsort: () => void;
    filtersActive: boolean;
    onclearfilters: () => void;
    refreshing: boolean;
    visibleCount: number;
    onselectall: () => void;
    /** Shown only when provided (Siril available, Frames tab). */
    onsuggest?: () => void;
    columns: ColumnDef[];
    ontogglecolumn: (colId: string) => void;
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
    sortActive,
    onclearsort,
    filtersActive,
    onclearfilters,
    refreshing,
    visibleCount,
    onselectall,
    onsuggest,
    columns,
    ontogglecolumn,
    shortcutsOpen,
    onshortcuts,
  }: Props = $props();

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
  <div class="tb-group" role="tablist" aria-label="Library view">
    <button
      class="seg-btn"
      role="tab"
      aria-selected={!showRejected}
      class:active={!showRejected}
      onclick={() => ontab(false)}>Frames</button
    >
    <button
      class="seg-btn"
      role="tab"
      aria-selected={showRejected}
      class:active={showRejected}
      onclick={() => ontab(true)}
    >
      Rejected
      {#if rejectedCount > 0}<span class="tab-badge">{rejectedCount}</span>{/if}
    </button>
  </div>

  <div class="tb-group">
    <span class="tb-label" id="lib-groupby-label">Group</span>
    <div class="segmented" role="group" aria-labelledby="lib-groupby-label">
      {#each GROUP_BY_OPTIONS as opt (opt.value)}
        <button
          class="seg-btn"
          class:active={groupBy === opt.value}
          aria-pressed={groupBy === opt.value}
          onclick={() => ongroupby(opt.value)}>{opt.label}</button
        >
      {/each}
    </div>
    <button class="tool-btn icon" onclick={onexpandall} title="Expand all groups">⊞</button>
    <button class="tool-btn icon" onclick={oncollapseall} title="Collapse all groups">⊟</button>
  </div>

  <input
    class="search-input"
    type="search"
    placeholder="Search name, object, filter…"
    aria-label="Search frames"
    spellcheck="false"
    bind:value={search}
  />

  {#if sortActive}
    <button class="tool-btn chip" onclick={onclearsort} title="Clear sort">✕ Sort</button>
  {/if}
  {#if filtersActive}
    <button class="tool-btn chip" onclick={onclearfilters} title="Clear search and column filters"
      >✕ Filters</button
    >
  {/if}

  <span class="tb-spacer"></span>

  {#if refreshing}
    <span class="refreshing" role="status"><span class="mini-spinner"></span>Refreshing…</span>
  {/if}

  {#if onsuggest && !showRejected}
    <button class="tool-btn" onclick={onsuggest} title="Suggest statistical outliers for rejection">
      ✦ Suggest rejects
    </button>
  {/if}
  <button
    class="tool-btn"
    onclick={onselectall}
    disabled={visibleCount === 0}
    title="Select all {visibleCount} frames in this view (Ctrl+A)"
  >
    Select all
  </button>
  <div class="column-selector" bind:this={colMenuRoot}>
    <button
      class="tool-btn"
      class:active={showColumnMenu}
      aria-expanded={showColumnMenu}
      onclick={() => (showColumnMenu = !showColumnMenu)}
      title="Show/hide columns">Columns ▾</button
    >
    {#if showColumnMenu}
      <div class="column-menu">
        {#each sortedColumns as col (col.id)}
          <label class="column-menu-item">
            <input type="checkbox" checked={col.visible} onchange={() => ontogglecolumn(col.id)} />
            {col.label}
          </label>
        {/each}
      </div>
    {/if}
  </div>
  <button
    class="tool-btn"
    data-shortcuts-toggle
    class:active={shortcutsOpen}
    aria-expanded={shortcutsOpen}
    onclick={onshortcuts}
    title="Keyboard shortcuts (?)">⌨ Shortcuts</button
  >
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px 10px;
    padding: 6px 10px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .tb-group {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }

  .tb-label {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    margin-right: 2px;
  }

  .tb-spacer {
    flex: 1;
  }

  .segmented {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 4px;
    overflow: hidden;
  }
  .segmented .seg-btn {
    border: none;
    border-radius: 0;
    border-right: 1px solid var(--border);
  }
  .segmented .seg-btn:last-child {
    border-right: none;
  }

  .seg-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 3px 10px;
    font-size: var(--fs-xs);
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    transition:
      background 0.12s,
      color 0.12s;
  }
  .seg-btn:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .seg-btn.active,
  .seg-btn.active:hover {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
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

  .tool-btn.icon {
    padding: 1px 6px;
    font-size: var(--fs-sm);
  }
  .tool-btn:disabled {
    color: var(--text-dim);
    border-color: var(--border);
    cursor: default;
  }

  .chip {
    color: var(--accent);
    border-color: var(--accent);
  }

  .search-input {
    font-size: var(--fs-sm);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    padding: 3px 8px;
    width: 210px;
    min-width: 120px;
    outline: none;
  }
  .search-input::placeholder {
    color: var(--text-dim);
  }
  .search-input:focus {
    border-color: var(--accent);
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
    min-width: 150px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.45);
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
</style>
