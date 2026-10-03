<script lang="ts">
  import type * as app from "$models/app";
  import type { ColumnDef, FileGroup, ViewMode } from "../lib/types";
  import { isFits, getCellValue } from "../lib/utils";
  import { makeColumnManager } from "../lib/columnManager";
  import { SvelteMap } from "svelte/reactivity";
  import { untrack } from "svelte";

  interface Props {
    files: app.EnrichedFileEntry[];
    selectedEntry: app.EnrichedFileEntry | null;
    columns: ColumnDef[];
    error: string;
    loading: boolean;
    onfileclick: (entry: app.EnrichedFileEntry) => void;
    oncontextmenu: (x: number, y: number, entry: app.EnrichedFileEntry) => void;
    onsavecolumns: () => void;
    onfilteredcountchange: (count: number) => void;
    /** Identifies the listed folder; the filter dropdown resets when it changes. */
    dirKey?: string;
  }

  let {
    files,
    selectedEntry,
    columns,
    error,
    loading,
    onfileclick,
    oncontextmenu,
    onsavecolumns,
    onfilteredcountchange,
    dirKey = "",
  }: Props = $props();

  // ── Local UI state ────────────────────────────────────────────────────────
  let viewMode = $state<ViewMode>("files");
  let searchQuery = $state("");
  let filterFilter = $state("");
  let groupByObject = $state(false);
  let showColumnMenu = $state(false);

  let dragOverIndex = $state(-1);
  const colMgr = makeColumnManager(
    () => columns,
    (i) => {
      dragOverIndex = i;
    },
    () => onsavecolumns(),
  );

  // ── Derived ───────────────────────────────────────────────────────────────
  let visibleColumns = $derived(
    [...columns].filter((c) => c.visible).sort((a, b) => a.order - b.order),
  );

  let totalColWidth = $derived(visibleColumns.reduce((s, c) => s + c.width, 0));

  let uniqueFilters = $derived(
    [...new Set(files.filter((f) => !f.isDir && f.filter).map((f) => f.filter))].sort(),
  );

  let rejectedCount = $derived(files.filter((f) => !f.isDir && f.isRejected).length);
  let filteredFiles = $derived(applyFilters(files, searchQuery, filterFilter, viewMode));
  let filteredDirs = $derived(filteredFiles.filter((f) => f.isDir));
  let filteredFits = $derived(filteredFiles.filter((f) => !f.isDir && f.hasMeta));
  let filteredOther = $derived(filteredFiles.filter((f) => !f.isDir && !f.hasMeta));
  let fileGroups = $derived(groupByObject ? buildGroups(filteredFits) : null);
  let filtersActive = $derived(searchQuery !== "" || filterFilter !== "");

  // A filter value from another folder would hide every file here (and its <select>
  // may not even render), so start each folder unfiltered.
  $effect(() => {
    void dirKey;
    untrack(() => {
      filterFilter = "";
    });
  });

  function clearFilters() {
    searchQuery = "";
    filterFilter = "";
  }

  $effect(() => {
    onfilteredcountchange(filteredFiles.length);
  });

  // Close column menu on outside click or Escape.
  let columnMenuRoot = $state<HTMLDivElement | null>(null);
  $effect(() => {
    if (!showColumnMenu) return;
    function onDoc(e: MouseEvent) {
      if (columnMenuRoot && !columnMenuRoot.contains(e.target as Node)) showColumnMenu = false;
    }
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      e.preventDefault();
      e.stopPropagation();
      showColumnMenu = false;
    }
    document.addEventListener("mousedown", onDoc);
    window.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      window.removeEventListener("keydown", onKey, true);
    };
  });

  // ── Filtering / grouping ─────────────────────────────────────────────────
  function applyFilters(
    all: app.EnrichedFileEntry[],
    search: string,
    filter: string,
    mode: ViewMode,
  ): app.EnrichedFileEntry[] {
    let r = all;
    if (mode === "files") {
      r = r.filter((f) => f.isDir || !f.isRejected);
    } else {
      r = r.filter((f) => !f.isDir && f.isRejected);
    }
    if (search) {
      const q = search.toLowerCase();
      r = r.filter(
        (f) => f.name.toLowerCase().includes(q) || (f.object && f.object.toLowerCase().includes(q)),
      );
    }
    if (filter) {
      r = r.filter((f) => f.isDir || f.filter === filter);
    }
    return r;
  }

  function buildGroups(fits: app.EnrichedFileEntry[]): FileGroup[] {
    const map = new SvelteMap<string, FileGroup>();
    for (const f of fits) {
      const obj = f.object || "—";
      const date = f.dateObs ? f.dateObs.substring(0, 10) : "—";
      const key = `${obj}|${date}`;
      if (!map.has(key)) map.set(key, { key, object: obj, date, files: [] });
      map.get(key)!.files.push(f);
    }
    return [...map.values()].sort((a, b) =>
      a.object !== b.object ? a.object.localeCompare(b.object) : a.date.localeCompare(b.date),
    );
  }

  function toggleColumn(colId: string) {
    const col = columns.find((c) => c.id === colId)!;
    col.visible = !col.visible;
    onsavecolumns();
  }
</script>

<!-- ── Table toolbar ──────────────────────────────────────────────────────── -->
<div class="table-toolbar">
  <div class="view-tabs">
    <button
      class="view-tab"
      class:active={viewMode === "files"}
      onclick={() => {
        viewMode = "files";
      }}>Files</button
    >
    <button
      class="view-tab"
      class:active={viewMode === "rejected"}
      onclick={() => {
        viewMode = "rejected";
      }}
      >Rejected{#if rejectedCount > 0}
        <span class="tab-badge">{rejectedCount}</span>{/if}</button
    >
  </div>
  <input
    class="search-input"
    type="search"
    placeholder="Search name or object…"
    bind:value={searchQuery}
  />
  {#if uniqueFilters.length > 0 || filterFilter}
    <select class="filter-select" bind:value={filterFilter} aria-label="Filter by optical filter">
      <option value="">All filters</option>
      {#each uniqueFilters as f (f)}
        <option value={f}>{f}</option>
      {/each}
      {#if filterFilter && !uniqueFilters.includes(filterFilter)}
        <option value={filterFilter}>{filterFilter}</option>
      {/if}
    </select>
  {/if}
  <button
    class="tool-btn"
    class:active={groupByObject}
    onclick={() => (groupByObject = !groupByObject)}
    title="Group by object + date">Group</button
  >
  <div class="column-selector" bind:this={columnMenuRoot}>
    <button
      class="tool-btn"
      onclick={() => (showColumnMenu = !showColumnMenu)}
      aria-expanded={showColumnMenu}
      title="Show/hide columns">Cols ▾</button
    >
    {#if showColumnMenu}
      <div class="column-menu">
        {#each [...columns].sort((a, b) => a.order - b.order) as col (col.id)}
          {#if col.id !== "name"}
            <label class="column-menu-item">
              <input type="checkbox" checked={col.visible} onchange={() => toggleColumn(col.id)} />
              {col.label}
            </label>
          {/if}
        {/each}
      </div>
    {/if}
  </div>
</div>

{#if error}
  <div class="error-bar">{error}</div>
{/if}

<!-- ── File table ──────────────────────────────────────────────────────────── -->
{#if loading}
  <div class="status-row">Loading…</div>
{:else if files.length === 0}
  <div class="status-row">This folder is empty</div>
{:else if filteredFiles.length === 0}
  <div class="status-row empty-filtered">
    {#if filtersActive}
      <span>No files match</span>
      <button class="btn-secondary" onclick={clearFilters}>Clear filters</button>
    {:else if viewMode === "rejected"}
      <span>No rejected files in this folder</span>
      <button class="btn-secondary" onclick={() => (viewMode = "files")}>Show files</button>
    {:else}
      <span>No files to show</span>
    {/if}
  </div>
{:else}
  <div class="table-scroll-wrapper">
    <table class="file-table" style="width: {Math.max(totalColWidth, 100)}px; min-width: 100%">
      <colgroup>
        {#each visibleColumns as col (col.id)}
          <col style="width: {col.width}px" />
        {/each}
      </colgroup>
      <thead>
        <tr>
          {#each visibleColumns as col, i (col.id)}
            <th
              class:drag-over={dragOverIndex === i}
              draggable={col.id !== "name"}
              ondragstart={(e) => colMgr.onColDragStart(e, i)}
              ondragover={(e) => colMgr.onColDragOver(e, i)}
              ondrop={(e) => colMgr.onColDrop(e, i)}
              ondragend={colMgr.onColDragEnd}
              ondragleave={() => {
                if (dragOverIndex === i) dragOverIndex = -1;
              }}
            >
              <span class="th-text">{col.label}</span>
              <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
              <span
                class="resize-handle"
                onmousedown={(e) => colMgr.startColResize(e, col.id)}
                role="separator"
                aria-label="Resize column"
              ></span>
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#if !groupByObject}
          {#each filteredFiles as entry (entry.path)}
            <tr
              class="file-row"
              class:is-dir={entry.isDir}
              class:is-fits={!entry.isDir && isFits(entry.name)}
              class:is-rejected={entry.isRejected}
              class:selected={selectedEntry && selectedEntry.path === entry.path}
              onclick={() => onfileclick(entry)}
              oncontextmenu={(e) => {
                e.preventDefault();
                oncontextmenu(e.clientX, e.clientY, entry);
              }}
            >
              {#each visibleColumns as col (col.id)}
                <td class="col-{col.id}">
                  {#if col.id === "name"}
                    <span class="file-icon"
                      >{entry.isDir ? "📁" : isFits(entry.name) ? "🔭" : "🗒"}</span
                    >
                    <span class="file-name">{entry.name}</span>
                  {:else}
                    {getCellValue(entry, col.id)}
                  {/if}
                </td>
              {/each}
            </tr>
          {/each}
        {:else}
          <!-- Grouped view -->
          {#each filteredDirs as entry (entry.path)}
            <tr
              class="file-row is-dir"
              onclick={() => onfileclick(entry)}
              oncontextmenu={(e) => {
                e.preventDefault();
                oncontextmenu(e.clientX, e.clientY, entry);
              }}
            >
              {#each visibleColumns as col (col.id)}
                <td class="col-{col.id}">
                  {#if col.id === "name"}
                    <span class="file-icon">📁</span>
                    <span class="file-name">{entry.name}</span>
                  {:else}
                    {getCellValue(entry, col.id)}
                  {/if}
                </td>
              {/each}
            </tr>
          {/each}

          {#each filteredOther as entry (entry.path)}
            <tr
              class="file-row"
              class:is-fits={isFits(entry.name)}
              class:is-rejected={entry.isRejected}
              class:selected={selectedEntry && selectedEntry.path === entry.path}
              onclick={() => onfileclick(entry)}
              oncontextmenu={(e) => {
                e.preventDefault();
                oncontextmenu(e.clientX, e.clientY, entry);
              }}
            >
              {#each visibleColumns as col (col.id)}
                <td class="col-{col.id}">
                  {#if col.id === "name"}
                    <span class="file-icon">{isFits(entry.name) ? "🔭" : "🗒"}</span>
                    <span class="file-name">{entry.name}</span>
                  {:else}
                    {getCellValue(entry, col.id)}
                  {/if}
                </td>
              {/each}
            </tr>
          {/each}

          {#each fileGroups ?? [] as group (group.key)}
            <tr class="group-header-row">
              <td colspan={visibleColumns.length}>
                <span class="group-object">{group.object}</span>
                <span class="group-date">{group.date}</span>
                <span class="group-count"
                  >{group.files.length} frame{group.files.length !== 1 ? "s" : ""}</span
                >
              </td>
            </tr>
            {#each group.files as entry (entry.path)}
              <tr
                class="file-row is-fits"
                class:is-rejected={entry.isRejected}
                class:selected={selectedEntry && selectedEntry.path === entry.path}
                onclick={() => onfileclick(entry)}
                oncontextmenu={(e) => {
                  e.preventDefault();
                  oncontextmenu(e.clientX, e.clientY, entry);
                }}
              >
                {#each visibleColumns as col (col.id)}
                  <td class="col-{col.id}">
                    {#if col.id === "name"}
                      <span class="file-icon">🔭</span>
                      <span class="file-name">{entry.name}</span>
                    {:else}
                      {getCellValue(entry, col.id)}
                    {/if}
                  </td>
                {/each}
              </tr>
            {/each}
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
{/if}

<style>
  /* ── Toolbar ──────────────────────────────────────────────────────────────── */

  .table-toolbar {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 5px 8px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .search-input {
    flex: 1;
    min-width: 0;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 3px 8px;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    outline: none;
  }
  .search-input:focus {
    border-color: var(--accent);
  }
  .search-input::placeholder {
    color: var(--text-secondary);
  }

  .filter-select {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 2px 6px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    cursor: pointer;
    outline: none;
    max-width: 110px;
  }
  .filter-select:focus {
    border-color: var(--accent);
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
    min-width: 130px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.45);
  }

  .column-menu-item {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 4px 10px;
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

  /* ── View tabs ──────────────────────────────────────────────────────────── */

  .view-tabs {
    display: flex;
    gap: 2px;
    flex-shrink: 0;
  }

  .view-tab {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid transparent;
    border-radius: 4px;
    padding: 2px 10px;
    font-size: var(--fs-xs);
    cursor: pointer;
    transition:
      color 0.15s,
      border-color 0.15s;
    display: flex;
    align-items: center;
    gap: 5px;
  }
  .view-tab:hover {
    color: var(--text-primary);
  }
  .view-tab.active {
    color: var(--accent-contrast);
    border-color: var(--accent);
    background: var(--accent);
  }

  .tab-badge {
    background: var(--danger);
    color: var(--bg-base);
    border-radius: 8px;
    padding: 0 5px;
    font-size: var(--fs-xs);
    font-weight: 600;
    line-height: 1.3;
  }

  /* ── Table scroll ───────────────────────────────────────────────────────── */

  .table-scroll-wrapper {
    flex: 1;
    overflow: auto;
  }
  .table-scroll-wrapper::-webkit-scrollbar {
    width: 6px;
    height: 6px;
  }
  .table-scroll-wrapper::-webkit-scrollbar-track {
    background: var(--bg-base);
  }
  .table-scroll-wrapper::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

  /* ── Table ──────────────────────────────────────────────────────────────── */

  .file-table {
    border-collapse: collapse;
    font-size: var(--fs-md);
    table-layout: fixed;
  }

  .file-table thead tr {
    background: var(--bg-panel);
    position: sticky;
    top: 0;
    z-index: 1;
  }

  .file-table th {
    padding: 7px 10px 7px 8px;
    text-align: left;
    font-size: var(--fs-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border);
    position: relative;
    overflow: hidden;
    white-space: nowrap;
    user-select: none;
  }

  .file-table th.drag-over {
    border-left: 2px solid var(--accent);
  }

  .th-text {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 6px;
  }

  .resize-handle {
    position: absolute;
    right: 0;
    top: 0;
    bottom: 0;
    width: 5px;
    cursor: col-resize;
    background: transparent;
    transition: background 0.1s;
  }
  .resize-handle:hover {
    background: var(--accent);
    opacity: 0.5;
  }

  .file-row td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-row:hover td {
    background: var(--bg-row-hover);
  }
  .file-row.is-dir {
    cursor: pointer;
  }
  .file-row.is-fits {
    cursor: pointer;
  }

  .file-row.is-dir td {
    background: var(--bg-row-dir);
    color: var(--accent);
  }
  .file-row.is-dir:hover td,
  .file-row.is-fits:hover td {
    background: var(--bg-row-hover);
  }

  .file-row.selected td {
    background: var(--accent-dim) !important;
    color: var(--text-primary);
  }

  .file-row.is-rejected td {
    color: var(--text-secondary);
    text-decoration: line-through;
  }

  .col-size,
  .col-expTime,
  .col-gain,
  .col-ccdTemp {
    text-align: right;
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }
  .col-modTime,
  .col-dateObs {
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }
  .col-filter,
  .col-object {
    font-size: var(--fs-sm);
  }

  .file-icon {
    margin-right: 6px;
    font-size: 0.9em;
  }

  .empty-filtered {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
  }
  .empty-filtered .btn-secondary {
    margin-right: 0;
  }

  /* ── Group header ───────────────────────────────────────────────────────── */

  .group-header-row td {
    background: color-mix(in srgb, var(--bg-panel) 85%, var(--accent) 15%);
    border-top: 1px solid var(--border-accent);
    border-bottom: 1px solid var(--border-accent);
    padding: 4px 10px;
  }

  .group-object {
    font-size: var(--fs-xs);
    font-weight: 600;
    color: var(--accent);
  }
  .group-date {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    margin-left: 8px;
    font-variant-numeric: tabular-nums;
  }
  .group-count {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    margin-left: 8px;
  }
</style>
