<script lang="ts">
  import type { ColumnDef } from "../../lib/types";
  import type { ColumnManager } from "../../lib/columnManager";
  import type { LibrarySort } from "../../lib/uiState.svelte";
  import {
    NUMERIC_FILTER_COLS,
    TEXT_FILTER_COLS,
    VALUE_PICK_COLS,
    isColFilterActive,
    type ColFilters,
  } from "../../lib/library/filters";

  interface Props {
    columns: ColumnDef[];
    sort: LibrarySort | null;
    colFilters: ColFilters;
    dragOverIndex: number;
    colMgr: ColumnManager;
    ontogglesort: (colId: string) => void;
    ondragleavecol: (index: number) => void;
    ontextfilter: (colId: string, text: string) => void;
    onnumfilter: (colId: string, val: number | null) => void;
    ontogglenumop: (colId: string) => void;
    /** Opens/closes the frame-type filter popup anchored at `rect`. */
    ontypefilter: (rect: DOMRect) => void;
    /** Opens/closes the value list for a column, anchored under its filter cell. */
    onpickvalues: (colId: string, rect: DOMRect) => void;
    /** Column whose value list is open. */
    pickerCol: string | null;
  }

  let {
    columns,
    sort,
    colFilters,
    dragOverIndex,
    colMgr,
    ontogglesort,
    ondragleavecol,
    ontextfilter,
    onnumfilter,
    ontogglenumop,
    ontypefilter,
    onpickvalues,
    pickerCol,
  }: Props = $props();

  const textTimers: Record<string, ReturnType<typeof setTimeout>> = {};

  function onTextInput(colId: string, value: string) {
    clearTimeout(textTimers[colId]);
    textTimers[colId] = setTimeout(() => ontextfilter(colId, value), 120);
  }

  // A resize drag ends with a click on the header; don't let it toggle sorting.
  let suppressSortClick = false;
  function startResize(e: MouseEvent, colId: string) {
    suppressSortClick = true;
    window.addEventListener("mouseup", () => setTimeout(() => (suppressSortClick = false), 0), {
      once: true,
    });
    colMgr.startColResize(e, colId);
  }

  let typeCount = $derived(colFilters.frameType?.types?.length ?? 0);
</script>

<thead>
  <tr>
    <th class="cb-th"><span class="visually-hidden">Selected</span></th>
    {#each columns as col, i (col.id)}
      <th
        class:drag-over={dragOverIndex === i}
        class:sorted={sort?.col === col.id}
        aria-sort={sort?.col === col.id
          ? sort.dir === "asc"
            ? "ascending"
            : "descending"
          : undefined}
        draggable={col.id !== "frameType" && col.id !== "name"}
        onclick={() => {
          if (!suppressSortClick) ontogglesort(col.id);
        }}
        ondragstart={(e) => colMgr.onColDragStart(e, i)}
        ondragover={(e) => colMgr.onColDragOver(e, i)}
        ondrop={(e) => colMgr.onColDrop(e, i)}
        ondragend={colMgr.onColDragEnd}
        ondragleave={() => ondragleavecol(i)}
        title="Sort by {col.label}"
      >
        <span class="th-inner">
          <span class="th-text">{col.label}</span>
          {#if isColFilterActive(colFilters, col.id)}
            <span class="th-indicator" title="Filter active">▽</span>
          {/if}
          {#if sort?.col === col.id}
            <span class="th-indicator">{sort.dir === "asc" ? "▲" : "▼"}</span>
          {/if}
        </span>
        <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
        <span
          class="resize-handle"
          onmousedown={(e) => startResize(e, col.id)}
          role="separator"
          aria-label="Resize column"
        ></span>
      </th>
    {/each}
  </tr>
  <tr class="filter-row">
    <th class="cb-th filter-th"></th>
    {#each columns as col (col.id)}
      <th class="filter-th">
        {#if col.id === "frameType"}
          <button
            class="filter-type-btn"
            data-type-filter-toggle
            class:filter-active={typeCount > 0}
            aria-label="Filter by frame type"
            onclick={(e) => ontypefilter((e.currentTarget as HTMLElement).getBoundingClientRect())}
          >
            {typeCount > 0 ? `${typeCount} ✓` : "All ▽"}
          </button>
        {:else if TEXT_FILTER_COLS.has(col.id)}
          <div class="filter-text-wrap">
            <input
              class="filter-text"
              class:filter-active={isColFilterActive(colFilters, col.id)}
              class:has-picker={VALUE_PICK_COLS.has(col.id)}
              type="text"
              placeholder="Filter…"
              aria-label="Filter {col.label}"
              spellcheck="false"
              value={colFilters[col.id]?.text ?? ""}
              oninput={(e) => onTextInput(col.id, (e.target as HTMLInputElement).value)}
              onkeydown={(e) => {
                if (e.key === "ArrowDown" && e.altKey && VALUE_PICK_COLS.has(col.id)) {
                  e.preventDefault();
                  const th = (e.currentTarget as HTMLElement).closest("th");
                  if (th) onpickvalues(col.id, th.getBoundingClientRect());
                }
              }}
            />
            {#if VALUE_PICK_COLS.has(col.id)}
              <button
                class="filter-pick-btn"
                class:open={pickerCol === col.id}
                data-value-picker-toggle
                aria-label="Choose {col.label.toLowerCase()} from the library"
                aria-expanded={pickerCol === col.id}
                title="Choose from the {col.label.toLowerCase()} values in the library (Alt+↓)"
                onclick={(e) => {
                  const th = (e.currentTarget as HTMLElement).closest("th");
                  if (th) onpickvalues(col.id, th.getBoundingClientRect());
                }}>▾</button
              >
            {/if}
          </div>
        {:else if NUMERIC_FILTER_COLS.has(col.id)}
          <div class="filter-num">
            <button
              class="filter-num-op"
              onclick={() => ontogglenumop(col.id)}
              aria-label="Toggle less than / greater than"
              title="Toggle < / >">{colFilters[col.id]?.numOp ?? "<"}</button
            >
            <input
              class="filter-num-val"
              class:filter-active={isColFilterActive(colFilters, col.id)}
              type="number"
              step="any"
              placeholder="—"
              aria-label="Filter {col.label}"
              value={colFilters[col.id]?.numVal ?? ""}
              oninput={(e) => {
                const v = parseFloat((e.target as HTMLInputElement).value);
                onnumfilter(col.id, isNaN(v) ? null : v);
              }}
            />
          </div>
        {/if}
      </th>
    {/each}
  </tr>
</thead>

<style>
  thead tr {
    background: var(--bg-panel);
  }

  th {
    padding: 7px 10px 7px 8px;
    text-align: left;
    font-size: var(--fs-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
    z-index: 2;
    background: var(--bg-panel);
    overflow: hidden;
    white-space: nowrap;
    user-select: none;
    cursor: pointer;
  }

  th.drag-over {
    border-left: 2px solid var(--accent);
  }

  th.sorted .th-text {
    color: var(--accent);
  }

  .cb-th {
    padding: 0;
    width: 28px;
    cursor: default;
  }

  .visually-hidden {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
  }

  .th-inner {
    display: flex;
    align-items: center;
    gap: 3px;
    min-width: 0;
    padding-right: 4px;
  }

  .th-text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .th-indicator {
    flex-shrink: 0;
    font-size: var(--fs-xs);
    color: var(--accent);
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
  }

  /* ── Filter row ──────────────────────────────────────────────────────────── */

  .filter-row th {
    padding: 3px 4px;
    background: color-mix(in srgb, var(--bg-panel) 55%, var(--bg-base) 45%);
    border-bottom: 2px solid var(--border-accent);
    /* Height of the label row above: padding + one line of --fs-xs + border. */
    top: calc(14px + var(--fs-xs) * 1.4 + 1px);
    cursor: default;
    text-transform: none;
    letter-spacing: normal;
    font-weight: normal;
  }

  .filter-text,
  .filter-num-val {
    width: 100%;
    min-width: 0;
    font-size: var(--fs-xs);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-primary);
    padding: 2px 4px;
    outline: none;
    box-sizing: border-box;
  }
  .filter-text::placeholder,
  .filter-num-val::placeholder {
    color: var(--text-dim);
  }
  .filter-text:focus,
  .filter-text.filter-active,
  .filter-num-val:focus,
  .filter-num-val.filter-active {
    border-color: var(--accent);
  }

  .filter-text-wrap {
    position: relative;
  }
  .filter-text.has-picker {
    padding-right: 18px;
  }
  .filter-pick-btn {
    position: absolute;
    top: 1px;
    right: 1px;
    bottom: 1px;
    width: 16px;
    padding: 0;
    font-size: var(--fs-xs);
    line-height: 1;
    background: transparent;
    border: none;
    border-left: 1px solid var(--border);
    border-radius: 0 2px 2px 0;
    color: var(--text-secondary);
    cursor: pointer;
  }
  .filter-pick-btn:hover,
  .filter-pick-btn.open {
    background: var(--bg-row-hover);
    color: var(--accent);
  }

  .filter-num {
    display: flex;
    gap: 2px;
    align-items: center;
  }

  .filter-num-op {
    font-size: var(--fs-xs);
    padding: 1px 5px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-secondary);
    cursor: pointer;
    flex-shrink: 0;
    font-family: monospace;
    line-height: 1.5;
  }
  .filter-num-op:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .filter-type-btn {
    font-size: var(--fs-xs);
    padding: 2px 5px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-secondary);
    cursor: pointer;
    width: 100%;
    text-align: center;
    white-space: nowrap;
    overflow: hidden;
  }
  .filter-type-btn.filter-active {
    border-color: var(--accent);
    color: var(--accent);
  }
  .filter-type-btn:hover {
    border-color: var(--accent);
    color: var(--text-primary);
  }
</style>
