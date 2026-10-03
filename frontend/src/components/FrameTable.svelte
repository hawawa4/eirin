<script lang="ts">
  import { untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import type * as app from "$models/app";
  import type { ColFilter, ColumnDef, FrameType } from "../lib/types";
  import { FRAME_TYPE_META, DEFAULT_LIBRARY_COLUMNS } from "../lib/types";
  import {
    getLibraryCellValue,
    getFrameTextVal,
    getFrameNumVal,
    getFrameSortVal,
  } from "../lib/utils";
  import Modal from "./Modal.svelte";

  interface Props {
    frames: app.LibraryFrame[];
    onselectionchange?: (paths: Set<string>) => void;
    /** Enables the "Remove N frames" button (asks for confirmation first). */
    onremove?: (paths: string[]) => void | Promise<void>;
    /** Explanation shown in the remove confirmation. */
    removeDescription?: string;
    onrowclick?: (frame: app.LibraryFrame) => void;
    hasMore?: boolean;
    loadingMore?: boolean;
    onloadmore?: () => void;
    /** Loads every remaining page (offered while `hasMore`). */
    onloadall?: () => void;
    hiddenColumns?: string[];
    resetKey?: unknown;
    /** Rows shown greyed out and not selectable (e.g. frames already in a project). */
    disabledPaths?: ReadonlySet<string>;
    disabledTitle?: string;
    /** false: no own scroll area — the table grows and the parent scrolls. */
    scroll?: boolean;
  }

  let {
    frames,
    onselectionchange,
    onremove,
    removeDescription = "The selected frames will be removed from this list.",
    onrowclick,
    hasMore = false,
    loadingMore = false,
    onloadmore,
    onloadall,
    hiddenColumns = [],
    resetKey = undefined,
    disabledPaths,
    disabledTitle = "Not selectable",
    scroll = true,
  }: Props = $props();

  // ── Columns ───────────────────────────────────────────────────────────────
  const VISIBLE_BY_DEFAULT = new Set([
    "frameType",
    "name",
    "object",
    "filter",
    "dateObs",
    "expTime",
  ]);
  let columns = $state<ColumnDef[]>(
    // Column set is fixed for the table's lifetime; only the initial hiddenColumns matter.
    untrack(() =>
      DEFAULT_LIBRARY_COLUMNS.filter((c) => !hiddenColumns.includes(c.id)).map((c) => ({
        ...c,
        visible: VISIBLE_BY_DEFAULT.has(c.id),
      })),
    ),
  );
  let showColumnMenu = $state(false);
  let colMenuEl = $state<HTMLDivElement | null>(null);
  let visibleColumns = $derived(
    [...columns].filter((c) => c.visible).sort((a, b) => a.order - b.order),
  );

  function toggleColumn(colId: string) {
    columns = columns.map((c) => (c.id === colId ? { ...c, visible: !c.visible } : c));
  }

  // ── Sort ──────────────────────────────────────────────────────────────────
  let sortCol = $state<string | null>(null);
  let sortDir = $state<"asc" | "desc">("asc");

  function toggleSort(colId: string) {
    if (sortCol === colId) sortDir = sortDir === "asc" ? "desc" : "asc";
    else {
      sortCol = colId;
      sortDir = "asc";
    }
  }

  // ── Column filters ────────────────────────────────────────────────────────
  const TEXT_COLS = new Set(["name", "object", "filter", "telescope", "instrument", "dateObs"]);
  const NUM_COLS = new Set([
    "expTime",
    "size",
    "gain",
    "ccdTemp",
    "fwhm",
    "starCount",
    "background",
    "noise",
    "snr",
  ]);

  let colFilters = $state<Record<string, ColFilter>>({});
  let typeFilterPos = $state<{ x: number; y: number } | null>(null);
  let typePopupEl = $state<HTMLDivElement | null>(null);
  let search = $state("");
  const debounceTimers: Record<string, ReturnType<typeof setTimeout>> = {};

  function isFilterActive(colId: string): boolean {
    const cf = colFilters[colId];
    if (!cf) return false;
    if (colId === "frameType") return (cf.types?.length ?? 0) > 0;
    if (TEXT_COLS.has(colId)) return !!cf.text;
    if (NUM_COLS.has(colId)) return cf.numOp != null && cf.numVal != null;
    return false;
  }
  let anyFilterActive = $derived(Object.keys(colFilters).some(isFilterActive));

  function setTextFilter(colId: string, text: string) {
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], text: text || undefined } };
  }
  function setTextDebounced(colId: string, text: string) {
    clearTimeout(debounceTimers[colId]);
    debounceTimers[colId] = setTimeout(() => setTextFilter(colId, text), 100);
  }
  function setNumFilter(colId: string, val: number | null) {
    const op = colFilters[colId]?.numOp ?? "<";
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], numOp: op, numVal: val } };
  }
  function toggleNumOp(colId: string) {
    const newOp: "<" | ">" = (colFilters[colId]?.numOp ?? "<") === "<" ? ">" : "<";
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], numOp: newOp } };
  }
  function toggleTypeFilter(type: FrameType) {
    const cur = colFilters["frameType"]?.types ?? [];
    const next = cur.includes(type) ? cur.filter((t) => t !== type) : [...cur, type];
    colFilters = { ...colFilters, frameType: { ...colFilters["frameType"], types: next } };
  }
  function clearAll() {
    colFilters = {};
    typeFilterPos = null;
    search = "";
  }

  /** A numeric cell with no value (not analysed / not recorded). */
  function isEmptyNum(colId: string, v: number | null): v is null {
    return v === null || Number.isNaN(v) || (colId === "expTime" && v === 0);
  }

  let filtered = $derived(
    frames.filter((f) => {
      if (search) {
        const q = search.toLowerCase();
        if (
          !f.fileName.toLowerCase().includes(q) &&
          !f.object.toLowerCase().includes(q) &&
          !f.filter.toLowerCase().includes(q)
        )
          return false;
      }
      for (const [colId, cf] of Object.entries(colFilters)) {
        if (colId === "frameType") {
          if (cf.types && cf.types.length > 0 && !cf.types.includes(f.frameType as FrameType))
            return false;
        } else if (TEXT_COLS.has(colId) && cf.text) {
          if (!getFrameTextVal(f, colId).toLowerCase().includes(cf.text.toLowerCase()))
            return false;
        } else if (NUM_COLS.has(colId) && cf.numOp && cf.numVal != null) {
          const v = getFrameNumVal(f, colId);
          // An active numeric filter hides rows that have no value for the column.
          if (isEmptyNum(colId, v)) return false;
          if (cf.numOp === "<" && v >= cf.numVal) return false;
          if (cf.numOp === ">" && v <= cf.numVal) return false;
        }
      }
      return true;
    }),
  );
  let sorted = $derived(
    sortCol
      ? [...filtered].sort((a, b) => {
          const av = getFrameSortVal(a, sortCol!),
            bv = getFrameSortVal(b, sortCol!);
          const m = sortDir === "asc" ? 1 : -1;
          return av < bv ? -m : av > bv ? m : 0;
        })
      : filtered,
  );

  function isDisabled(path: string): boolean {
    return disabledPaths?.has(path) ?? false;
  }
  let selectable = $derived(sorted.filter((f) => !isDisabled(f.nasPath)));

  // ── Selection ─────────────────────────────────────────────────────────────
  // eslint-disable-next-line svelte/no-unnecessary-state-wrap
  let selectedPaths = $state(new SvelteSet<string>());
  let lastSelectedPath = "";
  let allCheckEl = $state<HTMLInputElement | null>(null);

  $effect(() => {
    if (allCheckEl)
      allCheckEl.indeterminate = selectedPaths.size > 0 && selectedPaths.size < selectable.length;
  });

  function pick(next: SvelteSet<string>) {
    selectedPaths = next;
    onselectionchange?.(next);
  }

  // Never keep rows selected that the filters (or a reload) hid, or that became disabled:
  // actions on the selection must only touch what the user can see.
  $effect(() => {
    const visible = new Set(selectable.map((f) => f.nasPath));
    untrack(() => {
      if ([...selectedPaths].some((p) => !visible.has(p))) {
        pick(new SvelteSet<string>([...selectedPaths].filter((p) => visible.has(p))));
      }
    });
  });

  function handleRowClick(e: MouseEvent, frame: app.LibraryFrame) {
    if (isDisabled(frame.nasPath)) return;
    if (e.shiftKey && lastSelectedPath) {
      const aIdx = sorted.findIndex((f) => f.nasPath === lastSelectedPath);
      const bIdx = sorted.findIndex((f) => f.nasPath === frame.nasPath);
      if (aIdx !== -1 && bIdx !== -1) {
        const [lo, hi] = aIdx < bIdx ? [aIdx, bIdx] : [bIdx, aIdx];
        const next = new SvelteSet<string>(selectedPaths);
        for (let i = lo; i <= hi; i++) {
          if (!isDisabled(sorted[i].nasPath)) next.add(sorted[i].nasPath);
        }
        pick(next);
      }
      return;
    }
    if (e.ctrlKey || e.metaKey) {
      toggleCheckbox(frame);
      return;
    }
    pick(new SvelteSet<string>([frame.nasPath]));
    lastSelectedPath = frame.nasPath;
    onrowclick?.(frame);
  }

  function toggleCheckbox(frame: app.LibraryFrame) {
    if (isDisabled(frame.nasPath)) return;
    const next = new SvelteSet<string>(selectedPaths);
    if (next.has(frame.nasPath)) next.delete(frame.nasPath);
    else next.add(frame.nasPath);
    pick(next);
    lastSelectedPath = frame.nasPath;
  }

  function toggleAll() {
    pick(
      selectedPaths.size === selectable.length && selectable.length > 0
        ? new SvelteSet<string>()
        : new SvelteSet<string>(selectable.map((f) => f.nasPath)),
    );
  }

  // ── Remove (confirmed) ────────────────────────────────────────────────────
  let confirmRemove = $state<string[] | null>(null);
  let removing = $state(false);

  async function doRemove() {
    if (!onremove || !confirmRemove) return;
    removing = true;
    try {
      await onremove(confirmRemove);
      pick(new SvelteSet<string>());
    } catch {
      /* the parent reports the failure; keep the selection so the user can retry */
    } finally {
      removing = false;
      confirmRemove = null;
    }
  }

  // ── Effects ───────────────────────────────────────────────────────────────
  $effect(() => {
    if (!showColumnMenu) return;
    const onDoc = (e: MouseEvent) => {
      if (!colMenuEl?.contains(e.target as Node)) showColumnMenu = false;
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  });

  $effect(() => {
    if (!typeFilterPos) return;
    const onDoc = (e: MouseEvent) => {
      if (!typePopupEl?.contains(e.target as Node)) typeFilterPos = null;
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  });

  function typeMeta(t: string) {
    return (
      FRAME_TYPE_META[t] ?? {
        label: t,
        short: t.toUpperCase(),
        color: "var(--text-secondary)",
        bg: "var(--bg-row)",
      }
    );
  }

  // Reset selection whenever the parent passes a new resetKey value.
  // untrack prevents selectedPaths.clear() from adding a dependency back into this effect.
  $effect(() => {
    void resetKey;
    untrack(() => {
      selectedPaths.clear();
      onselectionchange?.(new SvelteSet());
    });
  });
</script>

<!-- ── Toolbar ─────────────────────────────────────────────────────────────── -->
<div class="ft-toolbar">
  {#if onremove && selectedPaths.size > 0}
    <button class="ft-btn ft-remove-btn" onclick={() => (confirmRemove = [...selectedPaths])}>
      Remove {selectedPaths.size} frame{selectedPaths.size !== 1 ? "s" : ""}…
    </button>
  {/if}
  {#if sortCol}
    <button
      class="ft-btn"
      onclick={() => {
        sortCol = null;
      }}>✕ sort</button
    >
  {/if}
  {#if anyFilterActive}
    <button class="ft-btn" onclick={clearAll}>✕ filters</button>
  {/if}
  <input
    class="ft-search"
    type="search"
    placeholder="Search…"
    aria-label="Search frames"
    bind:value={search}
  />
  <div class="ft-col-sel" bind:this={colMenuEl}>
    <button
      class="ft-btn"
      aria-expanded={showColumnMenu}
      onclick={() => (showColumnMenu = !showColumnMenu)}>Columns ▾</button
    >
    {#if showColumnMenu}
      <div class="ft-col-menu">
        {#each [...columns].sort((a, b) => a.order - b.order) as col (col.id)}
          {#if col.id !== "frameType" && col.id !== "name"}
            <label class="ft-col-item">
              <input type="checkbox" checked={col.visible} onchange={() => toggleColumn(col.id)} />
              {col.label}
            </label>
          {/if}
        {/each}
      </div>
    {/if}
  </div>
</div>

{#if hasMore}
  <div class="ft-partial">
    <span>
      {frames.length.toLocaleString()} frames loaded — search, filters and sort apply to loaded rows only.
    </span>
    {#if onloadall}
      <button class="ft-btn" disabled={loadingMore} onclick={onloadall}>
        {loadingMore ? "Loading…" : "Load all"}
      </button>
    {/if}
  </div>
{/if}

<!-- ── Table ──────────────────────────────────────────────────────────────── -->
<div class="ft-scroll" class:ft-noscroll={!scroll}>
  <table class="ft-table">
    <colgroup>
      <col style="width: 28px" />
      {#each visibleColumns as col (col.id)}<col style="width: {col.width}px" />{/each}
    </colgroup>
    <thead>
      <tr>
        <th class="ft-cb-th">
          <input
            type="checkbox"
            aria-label="Select all"
            bind:this={allCheckEl}
            disabled={selectable.length === 0}
            checked={selectedPaths.size === selectable.length && selectable.length > 0}
            onchange={toggleAll}
          />
        </th>
        {#each visibleColumns as col (col.id)}
          <th
            class:ft-sorted={sortCol === col.id}
            onclick={() => toggleSort(col.id)}
            aria-sort={sortCol === col.id
              ? sortDir === "asc"
                ? "ascending"
                : "descending"
              : undefined}
          >
            <span class="ft-th-text">{col.label}</span>
            {#if isFilterActive(col.id)}<span class="ft-filter-dot" title="Filtered">▽</span>{/if}
            {#if sortCol === col.id}<span class="ft-sort-ind">{sortDir === "asc" ? "▲" : "▼"}</span
              >{/if}
          </th>
        {/each}
      </tr>
      <tr class="ft-filter-row">
        <th class="ft-cb-th"></th>
        {#each visibleColumns as col (col.id)}
          <th class="ft-fth">
            {#if col.id === "frameType"}
              <button
                class="ft-ftype-btn"
                class:ft-factive={isFilterActive("frameType")}
                onclick={(e) => {
                  const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
                  typeFilterPos = typeFilterPos ? null : { x: r.left, y: r.bottom + 2 };
                }}
              >
                {(colFilters.frameType?.types?.length ?? 0) > 0
                  ? `${colFilters.frameType!.types!.length} ✓`
                  : "All ▽"}
              </button>
            {:else if TEXT_COLS.has(col.id)}
              <input
                class="ft-finput"
                class:ft-factive={isFilterActive(col.id)}
                type="text"
                placeholder="contains…"
                aria-label="Filter {col.label}"
                value={colFilters[col.id]?.text ?? ""}
                oninput={(e) => setTextDebounced(col.id, (e.target as HTMLInputElement).value)}
              />
            {:else if NUM_COLS.has(col.id)}
              <div class="ft-fnum">
                <button
                  class="ft-fnum-op"
                  title="Toggle less than / greater than"
                  onclick={() => toggleNumOp(col.id)}>{colFilters[col.id]?.numOp ?? "<"}</button
                >
                <input
                  class="ft-fnum-val"
                  class:ft-factive={isFilterActive(col.id)}
                  type="number"
                  step="any"
                  placeholder="—"
                  aria-label="Filter {col.label}"
                  value={colFilters[col.id]?.numVal ?? ""}
                  oninput={(e) => {
                    const v = parseFloat((e.target as HTMLInputElement).value);
                    setNumFilter(col.id, isNaN(v) ? null : v);
                  }}
                />
              </div>
            {/if}
          </th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#if sorted.length === 0}
        <tr class="ft-empty-row">
          <td colspan={visibleColumns.length + 1}>
            <div class="ft-empty">
              {frames.length === 0 ? "No frames." : "No frames match the current filter."}
              {#if anyFilterActive || search}
                <button class="ft-clear-btn" onclick={clearAll}>Remove filters</button>
              {/if}
            </div>
          </td>
        </tr>
      {/if}
      {#each sorted as frame (frame.nasPath)}
        {@const disabled = isDisabled(frame.nasPath)}
        <tr
          class="ft-row"
          class:ft-sel={selectedPaths.has(frame.nasPath)}
          class:ft-disabled={disabled}
          title={disabled ? disabledTitle : undefined}
          onclick={(e) => handleRowClick(e, frame)}
        >
          <td
            class="ft-cb-td"
            onclick={(e) => {
              e.stopPropagation();
              toggleCheckbox(frame);
            }}
          >
            <input
              type="checkbox"
              aria-label="Select {frame.fileName}"
              {disabled}
              checked={selectedPaths.has(frame.nasPath)}
              onclick={(e) => e.stopPropagation()}
              onchange={() => toggleCheckbox(frame)}
            />
          </td>
          {#each visibleColumns as col (col.id)}
            <td class="ft-col ft-col-{col.id}">
              {#if col.id === "frameType"}
                {@const m = typeMeta(frame.frameType)}
                <span class="ft-type-badge" style="color:{m.color};background:{m.bg}"
                  >{m.short}</span
                >
              {:else if col.id === "name"}
                <span class="ft-name">{frame.fileName}</span>
              {:else}
                {getLibraryCellValue(frame, col.id)}
              {/if}
            </td>
          {/each}
        </tr>
      {/each}
      {#if hasMore}
        <tr class="ft-load-row">
          <td colspan={visibleColumns.length + 1}>
            <button class="ft-load-btn" disabled={loadingMore} onclick={onloadmore}>
              {loadingMore ? "Loading…" : "Load more"}
            </button>
          </td>
        </tr>
      {/if}
    </tbody>
  </table>
</div>

{#if typeFilterPos}
  <div
    class="ft-type-popup"
    bind:this={typePopupEl}
    style="left:{typeFilterPos.x}px;top:{typeFilterPos.y}px"
  >
    {#each Object.entries(FRAME_TYPE_META) as [type, meta] (type)}
      <label class="ft-popup-item">
        <input
          type="checkbox"
          checked={colFilters.frameType?.types?.includes(type as FrameType) ?? false}
          onchange={() => toggleTypeFilter(type as FrameType)}
        />
        <span style="color:{meta.color}">{meta.label}</span>
      </label>
    {/each}
  </div>
{/if}

{#if confirmRemove}
  <Modal
    title="Remove {confirmRemove.length} frame{confirmRemove.length !== 1 ? 's' : ''}?"
    onclose={() => (confirmRemove = null)}
    busy={removing}
    danger
  >
    <p class="ft-confirm-text">{removeDescription}</p>
    {#snippet actions()}
      <button
        class="btn-ghost"
        data-autofocus
        disabled={removing}
        onclick={() => (confirmRemove = null)}>Cancel</button
      >
      <button class="btn-danger" disabled={removing} onclick={doRemove}>
        {removing ? "Removing…" : "Remove"}
      </button>
    {/snippet}
  </Modal>
{/if}

<style>
  /* ── Toolbar ─────────────────────────────────────────────────────────────── */
  .ft-toolbar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 8px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .ft-btn {
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    font-size: var(--fs-xs);
    padding: 2px 8px;
    cursor: pointer;
    white-space: nowrap;
    transition:
      background 0.1s,
      color 0.1s;
  }
  .ft-btn:hover:not(:disabled) {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .ft-btn:disabled {
    cursor: default;
    color: var(--text-dim);
  }

  .ft-remove-btn {
    color: var(--danger);
    border-color: var(--danger);
  }
  .ft-remove-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--danger) 15%, transparent);
    color: var(--danger);
  }

  .ft-search {
    font-size: var(--fs-sm);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    padding: 3px 8px;
    width: 160px;
    outline: none;
    margin-left: auto;
  }
  .ft-search:focus {
    border-color: var(--accent);
  }

  .ft-col-sel {
    position: relative;
    flex-shrink: 0;
  }

  .ft-col-menu {
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 5px 0;
    z-index: 200;
    min-width: 140px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.45);
  }

  .ft-col-item {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 4px 10px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
  }
  .ft-col-item:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .ft-col-item input {
    accent-color: var(--accent);
    cursor: pointer;
  }

  .ft-partial {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 10px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    background: color-mix(in srgb, var(--bg-panel) 70%, var(--accent-dim) 30%);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  /* ── Table ───────────────────────────────────────────────────────────────── */
  .ft-scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .ft-scroll.ft-noscroll {
    flex: none;
    overflow: visible;
  }
  .ft-scroll::-webkit-scrollbar {
    width: 6px;
    height: 6px;
  }
  .ft-scroll::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

  .ft-table {
    border-collapse: collapse;
    font-size: var(--fs-md);
    table-layout: fixed;
    width: 100%;
    min-width: 100%;
  }

  /* The whole header (titles + filter row) sticks to the nearest scroll area. */
  .ft-table thead {
    position: sticky;
    top: 0;
    z-index: 2;
    background: var(--bg-panel);
  }

  .ft-table th {
    padding: 7px 8px;
    text-align: left;
    font-size: var(--fs-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border);
    background: var(--bg-panel);
    white-space: nowrap;
    overflow: hidden;
    user-select: none;
    cursor: pointer;
  }

  .ft-th-text {
    display: inline-block;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: middle;
  }
  .ft-filter-dot,
  .ft-sort-ind {
    font-size: var(--fs-xs);
    color: var(--accent);
    margin-left: 3px;
    vertical-align: middle;
  }
  .ft-sorted .ft-th-text {
    color: var(--accent);
  }

  /* ── Checkbox column ─────────────────────────────────────────────────────── */
  .ft-cb-th {
    width: 28px !important;
    padding: 0 !important;
    text-align: center;
    cursor: default !important;
  }
  .ft-cb-th input,
  .ft-cb-td input {
    accent-color: var(--accent);
    cursor: pointer;
    width: 13px;
    height: 13px;
    vertical-align: middle;
  }
  .ft-cb-td {
    padding: 0 !important;
    text-align: center;
    cursor: default;
    width: 28px;
  }
  .ft-cb-td input:disabled,
  .ft-cb-th input:disabled {
    cursor: default;
  }

  /* ── Filter row ──────────────────────────────────────────────────────────── */
  .ft-filter-row th {
    padding: 2px 4px;
    background: color-mix(in srgb, var(--bg-panel) 55%, var(--bg-base) 45%);
    border-bottom: 2px solid var(--border-accent);
    cursor: default;
    text-transform: none;
    letter-spacing: normal;
  }
  .ft-fth {
    padding: 2px 4px !important;
    cursor: default !important;
  }

  .ft-finput {
    width: 100%;
    font-size: var(--fs-xs);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-primary);
    padding: 2px 4px;
    outline: none;
    box-sizing: border-box;
  }
  .ft-finput:focus,
  .ft-finput.ft-factive {
    border-color: var(--accent);
  }

  .ft-fnum {
    display: flex;
    gap: 2px;
    align-items: center;
  }
  .ft-fnum-op {
    font-size: var(--fs-xs);
    padding: 1px 4px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-secondary);
    cursor: pointer;
    flex-shrink: 0;
    font-family: monospace;
    line-height: 1.5;
  }
  .ft-fnum-op:hover {
    border-color: var(--accent);
    color: var(--accent);
  }
  .ft-fnum-val {
    width: 100%;
    min-width: 0;
    font-size: var(--fs-xs);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-primary);
    padding: 2px 3px;
    outline: none;
  }
  .ft-fnum-val:focus,
  .ft-fnum-val.ft-factive {
    border-color: var(--accent);
  }

  .ft-ftype-btn {
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
  .ft-ftype-btn.ft-factive {
    border-color: var(--accent);
    color: var(--accent);
  }
  .ft-ftype-btn:hover {
    border-color: var(--accent);
    color: var(--text-primary);
  }

  /* ── Rows ────────────────────────────────────────────────────────────────── */
  .ft-row {
    cursor: pointer;
  }
  .ft-row td {
    padding: 5px 8px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ft-row:hover td {
    background: var(--bg-row-hover);
  }
  .ft-row.ft-sel td {
    background: color-mix(in srgb, var(--accent-dim) 70%, var(--bg-base) 30%);
  }
  .ft-row.ft-sel:hover td {
    background: var(--accent-dim);
  }
  .ft-row.ft-disabled {
    cursor: default;
  }
  .ft-row.ft-disabled td,
  .ft-row.ft-disabled:hover td {
    background: var(--bg-base);
    color: var(--text-dim);
  }

  /* ── Column-specific ─────────────────────────────────────────────────────── */
  .ft-col-expTime,
  .ft-col-size,
  .ft-col-gain,
  .ft-col-ccdTemp,
  .ft-col-fwhm,
  .ft-col-starCount,
  .ft-col-background,
  .ft-col-noise,
  .ft-col-snr {
    text-align: right;
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }
  .ft-col-dateObs {
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }

  .ft-type-badge {
    display: inline-block;
    font-size: var(--fs-xs);
    font-weight: 700;
    font-family: monospace;
    padding: 1px 5px;
    border-radius: 3px;
    letter-spacing: 0.04em;
  }
  .ft-name {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* ── Empty ───────────────────────────────────────────────────────────────── */
  .ft-empty-row td {
    padding: 28px 0;
    text-align: center;
    color: var(--text-secondary);
  }
  .ft-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    font-size: var(--fs-md);
  }
  .ft-clear-btn {
    font-size: var(--fs-sm);
    padding: 4px 14px;
    background: transparent;
    border: 1px solid var(--accent);
    color: var(--accent);
    border-radius: 4px;
    cursor: pointer;
    transition:
      background 0.12s,
      color 0.12s;
  }
  .ft-clear-btn:hover {
    background: var(--accent);
    color: var(--accent-contrast);
  }

  /* ── Load more ───────────────────────────────────────────────────────────── */
  .ft-load-row td {
    padding: 10px;
    text-align: center;
    border-bottom: none;
  }
  .ft-load-btn {
    font-size: var(--fs-sm);
    padding: 4px 16px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    transition:
      background 0.1s,
      color 0.1s;
  }
  .ft-load-btn:hover:not(:disabled) {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .ft-load-btn:disabled {
    cursor: default;
    color: var(--text-dim);
  }

  /* ── Type filter popup ───────────────────────────────────────────────────── */
  .ft-type-popup {
    position: fixed;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 4px 0;
    z-index: 300;
    min-width: 130px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.5);
  }
  .ft-popup-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
  }
  .ft-popup-item:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .ft-popup-item input {
    accent-color: var(--accent);
    cursor: pointer;
  }

  .ft-confirm-text {
    margin: 0;
  }
</style>
