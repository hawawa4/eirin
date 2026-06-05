<script lang="ts">
  import { onMount } from "svelte";
  import { SvelteSet, SvelteMap } from "svelte/reactivity";
  import {
    GetLibraryFrames,
    SetFrameType,
    RejectFile,
    UnrejectFile,
    HardDeleteFile,
    BatchRejectFiles,
    BatchUnrejectFiles,
    BatchHardDeleteFiles,
    OpenWithSiril,
    AnalyzeFrames,
    CancelAnalysis,
    CreateProject,
    AddFramesToProject,
    SuggestRejects,
  } from "../../wailsjs/go/app/App.js";
  import { EventsOn } from "../../wailsjs/runtime/runtime.js";
  import type { app } from "../../wailsjs/go/models";
  import type {
    AnalysisProgress,
    ColFilter,
    ColumnDef,
    CtxEntry,
    CtxMenuState,
    FrameType,
    LibraryGroupBy,
    Project,
  } from "../lib/types";
  import { FRAME_TYPE_META } from "../lib/types";
  import {
    getLibraryCellValue,
    getFrameTextVal,
    getFrameNumVal,
    getFrameSortVal,
  } from "../lib/utils";
  import { makeColumnManager } from "../lib/columnManager";
  import ContextMenu from "./ContextMenu.svelte";
  import HardDeleteModal from "./HardDeleteModal.svelte";
  import BlinkModal from "./BlinkModal.svelte";

  interface Props {
    rootFolder: string;
    columns: ColumnDef[];
    selectedNasPath: string | null;
    sirilAvailable: boolean;
    initialFilter?: string;
    onfileclick: (frame: app.LibraryFrame) => void;
    onsavecolumns: () => void;
    onframesreloaded?: (frames: app.LibraryFrame[]) => void;
    oncreateproject?: (project: Project) => void;
  }

  let {
    rootFolder,
    columns,
    selectedNasPath,
    sirilAvailable,
    initialFilter,
    onfileclick,
    onsavecolumns,
    onframesreloaded,
    oncreateproject,
  }: Props = $props();

  // ── Data ─────────────────────────────────────────────────────────────────
  let frames = $state<app.LibraryFrame[]>([]);
  let loading = $state(false);
  let error = $state("");

  // ── Toolbar state ─────────────────────────────────────────────────────────
  let showRejected = $state(false);
  let groupBy = $state<LibraryGroupBy>("object");
  let search = $state("");
  let showColumnMenu = $state(false);

  // ── Sort state ────────────────────────────────────────────────────────────
  let sortCol = $state<string | null>(null);
  let sortDir = $state<"asc" | "desc">("asc");

  function toggleSort(colId: string) {
    if (sortCol === colId) {
      sortDir = sortDir === "asc" ? "desc" : "asc";
    } else {
      sortCol = colId;
      sortDir = "asc";
    }
  }

  // ── Column filters ────────────────────────────────────────────────────────
  const TEXT_FILTER_COLS = new Set([
    "name",
    "object",
    "filter",
    "telescope",
    "instrument",
    "dateObs",
  ]);
  const NUMERIC_FILTER_COLS = new Set([
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

  function getColFilterActive(colId: string): boolean {
    const cf = colFilters[colId];
    if (!cf) return false;
    if (colId === "frameType") return (cf.types?.length ?? 0) > 0;
    if (TEXT_FILTER_COLS.has(colId)) return !!cf.text;
    if (NUMERIC_FILTER_COLS.has(colId)) return cf.numOp != null && cf.numVal != null;
    return false;
  }

  let anyColFilterActive = $derived(Object.keys(colFilters).some((k) => getColFilterActive(k)));

  function clearColFilters() {
    colFilters = {};
    typeFilterPos = null;
  }

  function clearAllFilters() {
    clearColFilters();
    search = "";
  }

  // When the atlas opens a specific file, pre-filter the name column to that filename.
  $effect(() => {
    if (initialFilter) {
      colFilters = { name: { text: initialFilter } };
    }
  });

  const textDebounceTimers: Record<string, ReturnType<typeof setTimeout>> = {};

  function setTextFilter(colId: string, text: string) {
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], text: text || undefined } };
  }

  function setTextFilterDebounced(colId: string, text: string) {
    clearTimeout(textDebounceTimers[colId]);
    textDebounceTimers[colId] = setTimeout(() => setTextFilter(colId, text), 100);
  }

  function setNumFilter(colId: string, val: number | null) {
    const op = colFilters[colId]?.numOp ?? "<";
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], numOp: op, numVal: val } };
  }

  function toggleNumOp(colId: string) {
    const current = colFilters[colId]?.numOp ?? "<";
    const newOp: "<" | ">" = current === "<" ? ">" : "<";
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], numOp: newOp } };
  }

  function toggleTypeFilter(type: FrameType) {
    const current = colFilters["frameType"]?.types ?? [];
    const next = current.includes(type) ? current.filter((t) => t !== type) : [...current, type];
    colFilters = { ...colFilters, frameType: { ...colFilters["frameType"], types: next } };
  }

  // ── Column drag ───────────────────────────────────────────────────────────
  let dragOverIndex = $state(-1);
  const colMgr = makeColumnManager(
    () => columns,
    (i) => {
      dragOverIndex = i;
    },
    () => onsavecolumns(),
  );

  // ── Multi-select ──────────────────────────────────────────────────────────
  let selectedPaths = new SvelteSet<string>();
  let lastSelectedPath = "";
  let ctxPaths = $state<string[]>([]);

  // ── Context menu / delete modal ───────────────────────────────────────────
  let ctxMenu = $state<CtxMenuState | null>(null);
  let confirmDel = $state<{ paths: string[]; name: string } | null>(null);

  // ── Create project modal ──────────────────────────────────────────────────
  let cpModal = $state<{ paths: string[] } | null>(null);
  let cpName = $state("");
  let cpMode = $state<"symlink" | "copy">("symlink");
  let cpError = $state("");
  let cpCreating = $state(false);

  async function doCreateProject() {
    if (!cpName.trim() || !cpModal) return;
    cpCreating = true;
    cpError = "";
    try {
      const project = await CreateProject(cpName.trim(), "");
      await AddFramesToProject(project.folder, cpModal.paths, cpMode);
      cpModal = null;
      cpName = "";
      oncreateproject?.(project);
    } catch (e) {
      cpError = String(e);
    } finally {
      cpCreating = false;
    }
  }

  // ── Siril analysis ────────────────────────────────────────────────────────
  let analyzingGroup = $state<string | null>(null);
  let analysisProgress = $state<AnalysisProgress | null>(null);

  $effect(() => {
    const unsubProgress = EventsOn("analysis:progress", (data: AnalysisProgress) => {
      analysisProgress = data;
    });
    const unsubUpdated = EventsOn("library:updated", () => {
      reload();
    });
    return () => {
      unsubProgress();
      unsubUpdated();
    };
  });

  async function analyzeGroup(group: LibGroup) {
    const paths = group.frames.filter((f) => !f.qualityAnalyzed).map((f) => f.nasPath);
    if (!paths.length) return;
    analyzingGroup = group.key;
    analysisProgress = null;
    try {
      await AnalyzeFrames(paths);
    } finally {
      analyzingGroup = null;
      analysisProgress = null;
      reload();
    }
  }

  // ── Group collapse — empty set = all collapsed (default) ─────────────────
  let expandedGroups = new SvelteSet<string>();

  function toggleGroup(key: string) {
    if (expandedGroups.has(key)) expandedGroups.delete(key);
    else expandedGroups.add(key);
  }

  function expandAll() {
    expandedGroups.clear();
    groups.forEach((g) => expandedGroups.add(g.key));
  }

  function collapseAll() {
    expandedGroups.clear();
  }

  const GROUP_BY_OPTIONS: { value: LibraryGroupBy; label: string }[] = [
    { value: "object", label: "Object" },
    { value: "date", label: "Date" },
    { value: "filter", label: "Filter" },
    { value: "frameType", label: "Type" },
  ];

  onMount(() => {
    reload();
  });

  async function reload() {
    if (!rootFolder || loading) return;
    loading = true;
    error = "";
    selectedPaths.clear();
    try {
      frames = (await GetLibraryFrames(rootFolder)) ?? [];
      onframesreloaded?.(frames);
    } catch (e) {
      error = String(e);
      frames = [];
    } finally {
      loading = false;
    }
  }

  export { reload };

  // ── Derived ───────────────────────────────────────────────────────────────
  let visibleColumns = $derived(
    [...columns].filter((c) => c.visible).sort((a, b) => a.order - b.order),
  );

  let totalColWidth = $derived(visibleColumns.reduce((s, c) => s + c.width, 0));

  let rejectedCount = $derived(frames.filter((f) => f.isRejected).length);

  let filtered = $derived(
    frames.filter((f) => {
      if (showRejected ? !f.isRejected : f.isRejected) return false;
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
        } else if (TEXT_FILTER_COLS.has(colId) && cf.text) {
          if (!getFrameTextVal(f, colId).toLowerCase().startsWith(cf.text.toLowerCase()))
            return false;
        } else if (NUMERIC_FILTER_COLS.has(colId) && cf.numOp && cf.numVal != null) {
          const val = getFrameNumVal(f, colId);
          if (val === null) continue;
          if (cf.numOp === "<" && val >= cf.numVal) return false;
          if (cf.numOp === ">" && val <= cf.numVal) return false;
        }
      }
      return true;
    }),
  );

  let sorted = $derived(
    sortCol
      ? [...filtered].sort((a, b) => {
          const av = getFrameSortVal(a, sortCol!);
          const bv = getFrameSortVal(b, sortCol!);
          const mul = sortDir === "asc" ? 1 : -1;
          if (av < bv) return -1 * mul;
          if (av > bv) return 1 * mul;
          return 0;
        })
      : filtered,
  );

  interface LibGroup {
    key: string;
    label: string;
    frames: app.LibraryFrame[];
  }

  let groups = $derived<LibGroup[]>(buildGroups(sorted));

  function buildGroups(items: app.LibraryFrame[]): LibGroup[] {
    const map = new SvelteMap<string, app.LibraryFrame[]>();
    for (const f of items) {
      const key = groupKeyFor(f);
      const bucket = map.get(key);
      if (bucket) {
        bucket.push(f);
      } else {
        map.set(key, [f]);
      }
    }
    const result: LibGroup[] = [];
    for (const [key, gFrames] of map) {
      result.push({ key, label: labelForKey(key), frames: gFrames });
    }
    result.sort((a, b) => a.key.localeCompare(b.key));
    return result;
  }

  function groupKeyFor(f: app.LibraryFrame): string {
    switch (groupBy) {
      case "object":
        return f.object || "(unknown object)";
      case "date":
        return f.dateObs ? f.dateObs.slice(0, 10) : "(no date)";
      case "filter":
        return f.filter || "(no filter)";
      case "frameType":
        return f.frameType || "stacked";
    }
  }

  function labelForKey(key: string): string {
    if (groupBy === "frameType") return FRAME_TYPE_META[key]?.label ?? key;
    return key;
  }

  function frameTypeMeta(type: string) {
    return (
      FRAME_TYPE_META[type] ?? {
        label: type,
        short: type.toUpperCase(),
        color: "#9ca3af",
        bg: "#1f2937",
      }
    );
  }

  const TYPE_ORDER = ["light", "stacked", "processed", "flat", "dark", "bias"];

  function unanalyzedPaths(group: LibGroup): string[] {
    return group.frames.filter((f) => !f.qualityAnalyzed).map((f) => f.nasPath);
  }

  function groupTypeBreakdown(frames: app.LibraryFrame[]) {
    const counts = new SvelteMap<string, number>();
    for (const f of frames) counts.set(f.frameType, (counts.get(f.frameType) ?? 0) + 1);
    return [...counts.entries()]
      .sort((a, b) => {
        const ai = TYPE_ORDER.indexOf(a[0]);
        const bi = TYPE_ORDER.indexOf(b[0]);
        return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
      })
      .map(([type, count]) => ({ type, count, meta: frameTypeMeta(type) }));
  }

  function toggleColumn(colId: string) {
    const col = columns.find((c) => c.id === colId)!;
    col.visible = !col.visible;
    onsavecolumns();
  }

  // Close column menu on outside click
  $effect(() => {
    if (!showColumnMenu) return;
    function onDoc(e: MouseEvent) {
      const el = document.getElementById("lib-col-menu-root");
      if (el && !el.contains(e.target as Node)) showColumnMenu = false;
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  });

  // Close type-filter popup on outside click
  $effect(() => {
    if (!typeFilterPos) return;
    function onDoc(e: MouseEvent) {
      const el = document.getElementById("type-filter-popup");
      if (el && !el.contains(e.target as Node)) typeFilterPos = null;
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  });

  // ── Frame type change ─────────────────────────────────────────────────────
  async function changeFrameType(nasPath: string, newType: string, e: Event) {
    e.stopPropagation();
    await SetFrameType(nasPath, newType);
    frames = frames.map((f) => (f.nasPath === nasPath ? { ...f, frameType: newType } : f));
  }

  // ── Row click (single/ctrl/shift select) ─────────────────────────────────
  function handleRowClick(e: MouseEvent, frame: app.LibraryFrame) {
    if (e.shiftKey && lastSelectedPath) {
      const flat = sorted;
      const aIdx = flat.findIndex((f) => f.nasPath === lastSelectedPath);
      const bIdx = flat.findIndex((f) => f.nasPath === frame.nasPath);
      if (aIdx !== -1 && bIdx !== -1) {
        const [lo, hi] = aIdx < bIdx ? [aIdx, bIdx] : [bIdx, aIdx];
        for (let i = lo; i <= hi; i++) selectedPaths.add(flat[i].nasPath);
      }
      return;
    }
    if (e.ctrlKey || e.metaKey) {
      if (selectedPaths.has(frame.nasPath)) selectedPaths.delete(frame.nasPath);
      else selectedPaths.add(frame.nasPath);
      lastSelectedPath = frame.nasPath;
      return;
    }
    selectedPaths.clear();
    selectedPaths.add(frame.nasPath);
    lastSelectedPath = frame.nasPath;
    onfileclick(frame);
  }

  function handleCheckbox(frame: app.LibraryFrame) {
    if (selectedPaths.has(frame.nasPath)) selectedPaths.delete(frame.nasPath);
    else selectedPaths.add(frame.nasPath);
    lastSelectedPath = frame.nasPath;
  }

  // ── Reject / restore / hard delete ───────────────────────────────────────
  function openCtxMenu(e: MouseEvent, frame: app.LibraryFrame) {
    e.preventDefault();
    const inSelection = selectedPaths.has(frame.nasPath) && selectedPaths.size > 1;
    const paths = inSelection ? [...selectedPaths] : [frame.nasPath];
    ctxPaths = paths;
    ctxMenu = {
      x: e.clientX,
      y: e.clientY,
      entry: {
        path: frame.nasPath,
        name: frame.fileName,
        isRejected: frame.isRejected,
        frameType: frame.frameType,
      },
      sirilAvailable: sirilAvailable && paths.length === 1,
      selectionCount: paths.length,
    };
  }

  async function onCtxChangeType(entry: CtxEntry, newType: string) {
    ctxMenu = null;
    await SetFrameType(entry.path, newType);
    frames = frames.map((f) => (f.nasPath === entry.path ? { ...f, frameType: newType } : f));
  }

  async function onCtxOpenWithSiril(entry: CtxEntry) {
    ctxMenu = null;
    await OpenWithSiril(entry.path);
  }

  async function onCtxReject(entry: CtxEntry) {
    ctxMenu = null;
    if (ctxPaths.length > 1) {
      await BatchRejectFiles(ctxPaths);
      const set = new Set(ctxPaths);
      frames = frames.map((f) => (set.has(f.nasPath) ? { ...f, isRejected: true } : f));
      selectedPaths.clear();
    } else {
      await RejectFile(entry.path);
      frames = frames.map((f) => (f.nasPath === entry.path ? { ...f, isRejected: true } : f));
    }
  }

  async function onCtxRestore(entry: CtxEntry) {
    ctxMenu = null;
    if (ctxPaths.length > 1) {
      await BatchUnrejectFiles(ctxPaths);
      const set = new Set(ctxPaths);
      frames = frames.map((f) => (set.has(f.nasPath) ? { ...f, isRejected: false } : f));
      selectedPaths.clear();
    } else {
      await UnrejectFile(entry.path);
      frames = frames.map((f) => (f.nasPath === entry.path ? { ...f, isRejected: false } : f));
    }
  }

  function onCtxHardDelete(entry: CtxEntry) {
    ctxMenu = null;
    if (ctxPaths.length > 1) {
      confirmDel = { paths: [...ctxPaths], name: `${ctxPaths.length} frames` };
    } else {
      confirmDel = { paths: [entry.path], name: entry.name };
    }
  }

  async function doHardDelete() {
    if (!confirmDel) return;
    const { paths } = confirmDel;
    confirmDel = null;
    if (paths.length > 1) {
      await BatchHardDeleteFiles(paths);
      const set = new Set(paths);
      frames = frames.filter((f) => !set.has(f.nasPath));
      selectedPaths.clear();
    } else {
      await HardDeleteFile(paths[0]);
      frames = frames.filter((f) => f.nasPath !== paths[0]);
    }
  }

  // ── Blink comparison ──────────────────────────────────────────────────────
  let blinkFrames = $state<app.LibraryFrame[]>([]);
  let showBlink = $state(false);

  function openBlink() {
    const sel = [...selectedPaths];
    blinkFrames = frames.filter((f) => sel.includes(f.nasPath));
    if (blinkFrames.length >= 2) showBlink = true;
  }

  async function blinkReject(nasPath: string) {
    await RejectFile(nasPath);
    frames = frames.map((f) => (f.nasPath === nasPath ? { ...f, isRejected: true } : f));
    blinkFrames = blinkFrames.map((f) => (f.nasPath === nasPath ? { ...f, isRejected: true } : f));
  }

  function blinkHardDelete(nasPath: string, name: string) {
    confirmDel = { paths: [nasPath], name };
    // frame will be removed from blinkFrames via the reload after delete
  }

  // ── Smart reject suggestions ──────────────────────────────────────────────
  let suggestLoading = $state(false);
  let suggestResults = $state<import("../../wailsjs/go/models").app.SuggestResult[]>([]);
  let suggestSelected = $state(new Set<string>());
  let showSuggest = $state(false);

  async function openSuggest() {
    suggestLoading = true;
    showSuggest = true;
    suggestResults = [];
    const res = await SuggestRejects(rootFolder, 2.0);
    suggestResults = res ?? [];
    suggestSelected = new Set(suggestResults.map((r) => r.frame.nasPath));
    suggestLoading = false;
  }

  async function applySuggestRejects() {
    const paths = [...suggestSelected];
    if (!paths.length) return;
    await BatchRejectFiles(paths);
    const set = new Set(paths);
    frames = frames.map((f) => (set.has(f.nasPath) ? { ...f, isRejected: true } : f));
    showSuggest = false;
    suggestResults = [];
  }
</script>

<!-- ── Toolbar ───────────────────────────────────────────────────────────── -->
<div class="toolbar">
  <div class="view-tabs">
    <button
      class="view-tab"
      class:active={!showRejected}
      onclick={() => {
        showRejected = false;
      }}>Frames</button
    >
    <button
      class="view-tab"
      class:active={showRejected}
      onclick={() => {
        showRejected = true;
      }}
    >
      Rejected
      {#if rejectedCount > 0}<span class="tab-badge">{rejectedCount}</span>{/if}
    </button>
  </div>

  {#if !showRejected}
    <div class="group-by">
      <span class="label">Group</span>
      <div class="segmented">
        {#each GROUP_BY_OPTIONS as opt (opt.value)}
          <button
            class="seg-btn"
            class:active={groupBy === opt.value}
            onclick={() => {
              groupBy = opt.value;
            }}>{opt.label}</button
          >
        {/each}
      </div>
    </div>
  {/if}

  <div class="toolbar-sep"></div>

  <button class="tool-btn" onclick={expandAll} title="Expand all groups">⊞</button>
  <button class="tool-btn" onclick={collapseAll} title="Collapse all groups">⊟</button>
  {#if sortCol}
    <button
      class="tool-btn sort-clear"
      onclick={() => {
        sortCol = null;
      }}
      title="Clear sort"
    >
      ✕ sort
    </button>
  {/if}
  {#if anyColFilterActive}
    <button
      class="tool-btn filter-clear"
      onclick={clearColFilters}
      title="Clear all column filters"
    >
      ✕ filters
    </button>
  {/if}
  <button
    class="tool-btn blink-btn"
    disabled={selectedPaths.size < 2}
    onclick={openBlink}
    title={selectedPaths.size < 2
      ? "Select 2+ frames (checkboxes or Ctrl+click) to blink"
      : `Blink ${selectedPaths.size} selected frames`}
  >
    ▶ Blink{selectedPaths.size >= 2 ? ` (${selectedPaths.size})` : ""}
  </button>
  {#if !showRejected}
    <button
      class="tool-btn suggest-btn"
      onclick={openSuggest}
      title="Suggest statistical outliers for rejection"
    >
      ✦ Suggest rejects
    </button>
  {/if}
  <input class="search-input" type="search" placeholder="Search…" bind:value={search} />
  <div class="column-selector" id="lib-col-menu-root">
    <button
      class="tool-btn"
      onclick={() => (showColumnMenu = !showColumnMenu)}
      title="Show/hide columns">Cols ▾</button
    >
    {#if showColumnMenu}
      <div class="column-menu">
        {#each [...columns].sort((a, b) => a.order - b.order) as col (col.id)}
          {#if col.id !== "frameType" && col.id !== "name"}
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

<!-- ── Table ──────────────────────────────────────────────────────────────── -->
{#if loading}
  <div class="status-row">
    <span class="spinner" aria-label="Loading library"></span>
  </div>
{:else if error}
  <div class="status-row error">{error}</div>
{:else}
  <div class="table-scroll-wrapper">
    <table class="lib-table" style="width: {Math.max(totalColWidth + 28, 100)}px; min-width: 100%">
      <colgroup>
        <col style="width: 28px" />
        {#each visibleColumns as col (col.id)}
          <col style="width: {col.width}px" />
        {/each}
      </colgroup>
      <thead>
        <tr>
          <th class="cb-th" onclick={(e) => e.stopPropagation()}></th>
          {#each visibleColumns as col, i (col.id)}
            <th
              class:drag-over={dragOverIndex === i}
              class:sorted={sortCol === col.id}
              draggable={col.id !== "frameType" && col.id !== "name"}
              onclick={() => toggleSort(col.id)}
              ondragstart={(e) => colMgr.onColDragStart(e, i)}
              ondragover={(e) => colMgr.onColDragOver(e, i)}
              ondrop={(e) => colMgr.onColDrop(e, i)}
              ondragend={colMgr.onColDragEnd}
              ondragleave={() => {
                if (dragOverIndex === i) dragOverIndex = -1;
              }}
            >
              <span class="th-text">{col.label}</span>
              {#if getColFilterActive(col.id)}
                <span class="filter-indicator" title="Filter active">▽</span>
              {/if}
              {#if sortCol === col.id}
                <span class="sort-indicator">{sortDir === "asc" ? "▲" : "▼"}</span>
              {/if}
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
        <!-- Filter row -->
        <tr class="filter-row">
          <th class="cb-th filter-th"></th>
          {#each visibleColumns as col (col.id)}
            <th class="filter-th">
              {#if col.id === "frameType"}
                <button
                  class="filter-type-btn"
                  class:filter-active={getColFilterActive("frameType")}
                  onclick={(e) => {
                    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
                    typeFilterPos = typeFilterPos ? null : { x: rect.left, y: rect.bottom + 2 };
                  }}
                >
                  {#if (colFilters.frameType?.types?.length ?? 0) > 0}
                    {colFilters.frameType!.types!.length} ✓
                  {:else}
                    All ▽
                  {/if}
                </button>
              {:else if TEXT_FILTER_COLS.has(col.id)}
                <input
                  class="filter-text"
                  class:filter-active={getColFilterActive(col.id)}
                  type="text"
                  placeholder="…"
                  value={colFilters[col.id]?.text ?? ""}
                  oninput={(e) =>
                    setTextFilterDebounced(col.id, (e.target as HTMLInputElement).value)}
                />
              {:else if NUMERIC_FILTER_COLS.has(col.id)}
                <div class="filter-num">
                  <button
                    class="filter-num-op"
                    onclick={() => toggleNumOp(col.id)}
                    title="Toggle < / >">{colFilters[col.id]?.numOp ?? "<"}</button
                  >
                  <input
                    class="filter-num-val"
                    class:filter-active={getColFilterActive(col.id)}
                    type="number"
                    min="0"
                    step="any"
                    placeholder="—"
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
        {#if groups.length === 0}
          <tr class="empty-row">
            <td colspan={visibleColumns.length + 1}>
              <div class="empty-msg">
                {#if showRejected}
                  No rejected frames.
                {:else if frames.filter((f) => !f.isRejected).length === 0}
                  No indexed frames found. Run Build Index first.
                {:else}
                  No frames match the current filter.
                  {#if anyColFilterActive || search}
                    <button class="btn-clear-filters-inline" onclick={clearAllFilters}>
                      Remove filters
                    </button>
                  {/if}
                {/if}
              </div>
            </td>
          </tr>
        {/if}
        {#each groups as group (group.key)}
          <!-- Group header row -->
          <tr class="group-header-row" onclick={() => toggleGroup(group.key)}>
            <td colspan={visibleColumns.length + 1}>
              <div class="group-hdr-inner">
                <span class="group-chevron">{expandedGroups.has(group.key) ? "▼" : "▶"}</span>
                <span class="group-label">{group.label}</span>
                <span class="group-count"
                  >{group.frames.length} frame{group.frames.length !== 1 ? "s" : ""}</span
                >
                <span class="group-type-breakdown">
                  {#each groupTypeBreakdown(group.frames) as { count, meta } (meta.short)}
                    <span class="group-type-badge" style="color:{meta.color};background:{meta.bg}">
                      {meta.short}
                      {count}
                    </span>
                  {/each}
                </span>
                {#if group.frames.some((f) => f.qualityAnalyzed)}
                  <span class="quality-dot" title="Quality data available">✦</span>
                {/if}
                <span class="group-spacer"></span>
                {#if sirilAvailable && group.frames.some((f) => !f.qualityAnalyzed)}
                  {#if analyzingGroup === group.key}
                    <span class="analysis-status">
                      ⟳ {analysisProgress?.done ?? 0}/{analysisProgress?.total ??
                        unanalyzedPaths(group).length}
                      {#if analysisProgress?.current}· {analysisProgress.current}{/if}
                    </span>
                    <button
                      class="btn-cancel-analysis"
                      onclick={(e) => {
                        e.stopPropagation();
                        CancelAnalysis();
                      }}
                      title="Cancel analysis">✕</button
                    >
                  {:else}
                    <button
                      class="btn-analyze"
                      onclick={(e) => {
                        e.stopPropagation();
                        analyzeGroup(group);
                      }}
                      disabled={analyzingGroup !== null}
                      title="Analyze light frames with Siril (findstar)">✦ Analyze</button
                    >
                  {/if}
                {/if}
              </div>
            </td>
          </tr>

          <!-- Frame rows — only rendered when group is expanded -->
          {#if expandedGroups.has(group.key)}
            {#each group.frames as frame (frame.nasPath)}
              <tr
                class="frame-row"
                class:selected={selectedNasPath === frame.nasPath}
                class:multi-selected={selectedPaths.has(frame.nasPath)}
                onclick={(e) => handleRowClick(e, frame)}
                oncontextmenu={(e) => openCtxMenu(e, frame)}
              >
                <td
                  class="cb-td"
                  onclick={(e) => {
                    e.stopPropagation();
                    handleCheckbox(frame);
                  }}
                >
                  <input
                    type="checkbox"
                    checked={selectedPaths.has(frame.nasPath)}
                    onclick={(e) => e.stopPropagation()}
                    onchange={() => handleCheckbox(frame)}
                  />
                </td>
                {#each visibleColumns as col (col.id)}
                  <td class="col-{col.id}">
                    {#if col.id === "frameType"}
                      {@const meta = frameTypeMeta(frame.frameType)}
                      <select
                        class="type-select"
                        value={frame.frameType}
                        style="color:{meta.color};background:{meta.bg}"
                        onclick={(e) => e.stopPropagation()}
                        onchange={(e) =>
                          changeFrameType(frame.nasPath, (e.target as HTMLSelectElement).value, e)}
                      >
                        {#each Object.entries(FRAME_TYPE_META) as [val, m] (val)}
                          <option value={val}>{m.short}</option>
                        {/each}
                      </select>
                    {:else if col.id === "name"}
                      <span class="file-icon">🔭</span>
                      <span class="file-name">{frame.fileName}</span>
                    {:else}
                      {getLibraryCellValue(frame, col.id)}
                    {/if}
                  </td>
                {/each}
              </tr>
            {/each}
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if typeFilterPos}
  <div
    class="type-filter-popup"
    id="type-filter-popup"
    style="left: {typeFilterPos.x}px; top: {typeFilterPos.y}px"
  >
    {#each Object.entries(FRAME_TYPE_META) as [type, meta] (type)}
      <label class="filter-popup-item">
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

{#if ctxMenu}
  <ContextMenu
    menu={ctxMenu}
    onclose={() => {
      ctxMenu = null;
    }}
    onreject={onCtxReject}
    onrestore={onCtxRestore}
    onharddelete={onCtxHardDelete}
    onopensiril={onCtxOpenWithSiril}
    onchangetype={onCtxChangeType}
    oncreateproject={oncreateproject
      ? () => {
          ctxMenu = null;
          cpModal = { paths: [...ctxPaths] };
          cpName = "";
          cpMode = "symlink";
          cpError = "";
        }
      : undefined}
  />
{/if}

{#if cpModal}
  <div
    class="cp-backdrop"
    onclick={() => {
      cpModal = null;
    }}
    onkeydown={(e) => e.key === "Escape" && (cpModal = null)}
    role="presentation"
  >
    <div
      class="cp-modal"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <p class="cp-title">Create project</p>
      <p class="cp-sub">
        {cpModal.paths.length} frame{cpModal.paths.length !== 1 ? "s" : ""} will be added
      </p>
      <input
        class="cp-input"
        type="text"
        placeholder="Project name"
        bind:value={cpName}
        spellcheck="false"
        onkeydown={(e) => {
          if (e.key === "Enter") doCreateProject();
        }}
      />
      <div class="cp-mode">
        <label class="cp-mode-opt"
          ><input type="radio" name="cpMode" value="symlink" bind:group={cpMode} /> Symlink</label
        >
        <label class="cp-mode-opt"
          ><input type="radio" name="cpMode" value="copy" bind:group={cpMode} /> Copy</label
        >
      </div>
      {#if cpError}<p class="cp-error">{cpError}</p>{/if}
      <div class="cp-btns">
        <button
          class="cp-btn-primary"
          onclick={doCreateProject}
          disabled={cpCreating || !cpName.trim()}
        >
          {cpCreating ? "Creating…" : "Create project"}
        </button>
        <button
          class="cp-btn-ghost"
          onclick={() => {
            cpModal = null;
          }}>Cancel</button
        >
      </div>
    </div>
  </div>
{/if}

{#if confirmDel}
  <HardDeleteModal
    target={confirmDel}
    onconfirm={doHardDelete}
    oncancel={() => {
      confirmDel = null;
    }}
  />
{/if}

{#if showBlink && blinkFrames.length >= 2}
  <BlinkModal
    frames={blinkFrames}
    onclose={() => (showBlink = false)}
    onreject={blinkReject}
    onharddelete={blinkHardDelete}
  />
{/if}

{#if showSuggest}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="suggest-backdrop"
    onmousedown={(e) => {
      if (e.target === e.currentTarget) showSuggest = false;
    }}
  >
    <div class="suggest-modal">
      <div class="suggest-header">
        <span class="suggest-title">Suggested Rejects</span>
        <button class="suggest-close" onclick={() => (showSuggest = false)}>✕</button>
      </div>
      <div class="suggest-body">
        {#if suggestLoading}
          <p class="suggest-status">Analyzing quality metrics…</p>
        {:else if suggestResults.length === 0}
          <p class="suggest-status">
            No outliers found. Either all frames are good quality, or not enough frames have been
            analyzed (run ✦ Analyze first).
          </p>
        {:else}
          <p class="suggest-desc">
            {suggestResults.length} frame{suggestResults.length !== 1 ? "s" : ""} with FWHM &gt; 2σ above
            their group median. Deselect any you want to keep.
          </p>
          <div class="suggest-list">
            {#each suggestResults as r (r.frame.nasPath)}
              <label class="suggest-row" class:deselected={!suggestSelected.has(r.frame.nasPath)}>
                <input
                  type="checkbox"
                  checked={suggestSelected.has(r.frame.nasPath)}
                  onchange={() => {
                    const next = new Set(suggestSelected);
                    if (next.has(r.frame.nasPath)) next.delete(r.frame.nasPath);
                    else next.add(r.frame.nasPath);
                    suggestSelected = next;
                  }}
                />
                <span class="suggest-name">{r.frame.fileName}</span>
                <span class="suggest-obj">{r.frame.object}</span>
                <span
                  class="suggest-fwhm"
                  title="FWHM: {r.frame.fwhm.toFixed(2)} vs median {r.groupMedian.toFixed(
                    2,
                  )} (σ={r.groupSigma.toFixed(2)})"
                >
                  {r.frame.fwhm.toFixed(2)}
                  {r.frame.fwhmUnit} · {r.sigmas.toFixed(1)}σ
                </span>
              </label>
            {/each}
          </div>
        {/if}
      </div>
      {#if !suggestLoading && suggestResults.length > 0}
        <div class="suggest-footer">
          <span class="suggest-sel-count">{suggestSelected.size} selected</span>
          <button class="cp-btn-ghost" onclick={() => (showSuggest = false)}>Cancel</button>
          <button
            class="cp-btn-primary"
            disabled={suggestSelected.size === 0}
            onclick={applySuggestRejects}
          >
            Reject {suggestSelected.size} frame{suggestSelected.size !== 1 ? "s" : ""}
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  /* ── Toolbar ─────────────────────────────────────────────────────────────── */

  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    padding: 5px 8px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .toolbar-sep {
    width: 1px;
    height: 16px;
    background: var(--border);
    flex-shrink: 0;
    margin: 0 2px;
  }

  /* ── View tabs ───────────────────────────────────────────────────────────── */

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
    font-size: 0.75rem;
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
    color: var(--accent);
    border-color: var(--accent);
    background: var(--accent-dim);
  }

  .tab-badge {
    background: var(--danger);
    color: #fff;
    border-radius: 8px;
    padding: 0 5px;
    font-size: 0.65rem;
    font-weight: 600;
    line-height: 14px;
  }

  /* ── Group by ────────────────────────────────────────────────────────────── */

  .group-by {
    display: flex;
    align-items: center;
    gap: 5px;
  }

  .label {
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .segmented {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 4px;
    overflow: hidden;
  }

  .seg-btn {
    padding: 2px 8px;
    font-size: 0.75rem;
    background: transparent;
    border: none;
    color: var(--text-primary);
    cursor: pointer;
    border-right: 1px solid var(--border);
    transition:
      background 0.12s,
      color 0.12s;
  }

  .seg-btn:last-child {
    border-right: none;
  }

  .seg-btn:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }

  .seg-btn.active {
    background: var(--accent-dim);
    color: var(--accent);
  }

  /* ── Right side toolbar ──────────────────────────────────────────────────── */

  .search-input {
    font-size: 0.8rem;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    padding: 3px 8px;
    width: 150px;
    outline: none;
  }
  .search-input:focus {
    border-color: var(--accent);
  }

  .sort-clear {
    color: var(--accent);
    border-color: var(--accent);
    font-size: 0.72rem;
    white-space: nowrap;
  }

  /* ── Empty state inside table ────────────────────────────────────────────── */

  .empty-row td {
    padding: 32px 0;
    text-align: center;
    color: var(--text-secondary);
    font-size: 0.875rem;
  }

  .empty-msg {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
  }

  .btn-clear-filters-inline {
    font-size: 0.78rem;
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
  .btn-clear-filters-inline:hover {
    background: var(--accent);
    color: var(--bg-base);
  }

  .sort-indicator {
    font-size: 0.55rem;
    color: var(--accent);
    margin-left: 3px;
    flex-shrink: 0;
  }

  th.sorted .th-text {
    color: var(--accent);
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
    font-size: 0.82rem;
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

  /* ── Status ──────────────────────────────────────────────────────────────── */

  .status-row {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.875rem;
    color: var(--text-secondary);
  }
  .status-row.error {
    color: var(--danger);
  }
  .spinner {
    display: inline-block;
    width: 28px;
    height: 28px;
    border: 3px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* ── Table ───────────────────────────────────────────────────────────────── */

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

  .lib-table {
    border-collapse: collapse;
    font-size: 0.875rem;
    table-layout: fixed;
  }

  .lib-table thead tr {
    background: var(--bg-panel);
  }

  .lib-table th {
    padding: 7px 10px 7px 8px;
    text-align: left;
    font-size: 0.72rem;
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
  }

  .lib-table th.drag-over {
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

  /* ── Group header row ────────────────────────────────────────────────────── */

  .group-header-row {
    cursor: pointer;
    user-select: none;
  }

  .group-header-row:hover td {
    background: color-mix(in srgb, var(--bg-panel) 70%, var(--accent) 30%);
  }

  .group-header-row td {
    background: color-mix(in srgb, var(--bg-panel) 85%, var(--accent) 15%);
    border-top: 1px solid var(--border-accent);
    border-bottom: 1px solid var(--border-accent);
    padding: 4px 10px;
  }

  .group-hdr-inner {
    display: flex;
    align-items: center;
    width: 100%;
  }

  .group-chevron {
    font-size: 0.62rem;
    color: var(--text-secondary);
    margin-right: 6px;
    display: inline-block;
    width: 10px;
  }

  .group-label {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text-primary);
  }

  .group-count {
    font-size: 0.72rem;
    color: var(--text-secondary);
    margin-left: 8px;
  }

  .group-type-breakdown {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-left: 10px;
  }

  .group-type-badge {
    display: inline-flex;
    align-items: center;
    font-size: 0.66rem;
    font-weight: 700;
    font-family: monospace;
    padding: 1px 6px;
    border-radius: 3px;
    letter-spacing: 0.04em;
  }

  .group-spacer {
    flex: 1;
  }

  .quality-dot {
    font-size: 0.6rem;
    color: var(--accent);
    margin-left: 6px;
    opacity: 0.75;
  }

  .btn-analyze {
    font-size: 0.68rem;
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
    color: var(--bg-base);
  }

  .btn-analyze:disabled {
    opacity: 0.4;
    cursor: default;
  }

  .analysis-status {
    font-size: 0.68rem;
    color: var(--accent);
    white-space: nowrap;
    flex-shrink: 0;
    animation: pulse-opacity 1.2s ease-in-out infinite;
  }

  @keyframes pulse-opacity {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.5;
    }
  }

  .btn-cancel-analysis {
    font-size: 0.65rem;
    padding: 1px 5px;
    background: transparent;
    border: 1px solid var(--text-secondary);
    color: var(--text-secondary);
    border-radius: 3px;
    cursor: pointer;
    margin-left: 4px;
    flex-shrink: 0;
  }

  .btn-cancel-analysis:hover {
    border-color: #ef4444;
    color: #ef4444;
  }

  /* ── Checkbox column ─────────────────────────────────────────────────────── */

  .cb-th {
    padding: 0 !important;
    width: 28px;
    text-align: center;
  }

  .cb-td {
    padding: 0 !important;
    text-align: center;
    cursor: default;
    width: 28px;
  }

  .cb-td input[type="checkbox"] {
    accent-color: var(--accent);
    cursor: pointer;
    width: 13px;
    height: 13px;
    vertical-align: middle;
  }

  /* ── Frame rows ──────────────────────────────────────────────────────────── */

  .frame-row {
    cursor: pointer;
  }

  .frame-row td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .frame-row:hover td {
    background: var(--bg-row-hover);
  }

  .frame-row.selected td {
    background: var(--accent-dim) !important;
  }

  .frame-row.multi-selected td {
    background: color-mix(in srgb, var(--accent-dim) 70%, var(--bg-base) 30%);
  }

  .frame-row.multi-selected:hover td {
    background: var(--accent-dim);
  }

  /* ── Column-specific styles ──────────────────────────────────────────────── */

  .col-frameType {
    padding: 4px 6px !important;
    width: 70px;
  }

  .col-expTime,
  .col-size,
  .col-gain,
  .col-ccdTemp {
    text-align: right;
    color: var(--text-secondary);
    font-size: 0.82rem;
    font-variant-numeric: tabular-nums;
  }

  .col-dateObs {
    color: var(--text-secondary);
    font-size: 0.82rem;
    font-variant-numeric: tabular-nums;
  }

  .col-filter,
  .col-object {
    font-size: 0.83rem;
  }

  .file-icon {
    margin-right: 6px;
    font-size: 0.9em;
  }

  .file-name {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* ── Type select badge ───────────────────────────────────────────────────── */

  .type-select {
    appearance: none;
    -webkit-appearance: none;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 3px;
    font-size: 0.68rem;
    font-weight: 600;
    font-family: "Consolas", "Fira Code", monospace;
    padding: 2px 5px;
    cursor: pointer;
    width: 100%;
    text-align: center;
    outline: none;
    transition: border-color 0.12s;
  }

  .type-select:hover {
    border-color: rgba(255, 255, 255, 0.3);
  }

  .type-select option {
    background: var(--bg-panel);
    color: var(--text-primary);
    font-weight: normal;
  }

  /* ── Filter row ──────────────────────────────────────────────────────────── */

  .filter-row th {
    padding: 2px 4px;
    background: color-mix(in srgb, var(--bg-panel) 55%, var(--bg-base) 45%);
    border-bottom: 2px solid var(--border-accent);
    top: 31px;
  }

  .filter-text {
    width: 100%;
    font-size: 0.72rem;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-primary);
    padding: 2px 4px;
    outline: none;
    box-sizing: border-box;
  }
  .filter-text:focus,
  .filter-text.filter-active {
    border-color: var(--accent);
  }

  .filter-num {
    display: flex;
    gap: 2px;
    align-items: center;
  }

  .filter-num-op {
    font-size: 0.72rem;
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
  .filter-num-op:hover {
    border-color: var(--accent);
    color: var(--accent);
  }

  .filter-num-val {
    width: 100%;
    min-width: 0;
    font-size: 0.72rem;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-primary);
    padding: 2px 3px;
    outline: none;
  }
  .filter-num-val:focus,
  .filter-num-val.filter-active {
    border-color: var(--accent);
  }

  .filter-type-btn {
    font-size: 0.68rem;
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

  .filter-indicator {
    font-size: 0.5rem;
    color: var(--accent);
    margin-left: 2px;
    flex-shrink: 0;
    vertical-align: middle;
  }

  .filter-clear {
    color: var(--accent);
    border-color: var(--accent);
    font-size: 0.72rem;
    white-space: nowrap;
  }

  /* ── Type-filter popup (position: fixed, outside scroll wrapper) ─────────── */

  .type-filter-popup {
    position: fixed;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 4px 0;
    z-index: 300;
    min-width: 120px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.5);
  }

  .filter-popup-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    font-size: 0.8rem;
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
  }
  .filter-popup-item:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .filter-popup-item input {
    accent-color: var(--accent);
    cursor: pointer;
  }

  /* ── Create project modal ────────────────────────────────────────────────── */
  .cp-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2000;
  }
  .cp-modal {
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    padding: 22px 26px;
    width: 340px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.7);
  }
  .cp-title {
    font-size: 1rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }
  .cp-sub {
    font-size: 0.8rem;
    color: var(--text-secondary);
    margin: 0;
  }
  .cp-input {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    font-size: 0.9rem;
    padding: 6px 10px;
    outline: none;
    width: 100%;
    box-sizing: border-box;
  }
  .cp-input:focus {
    border-color: var(--accent);
  }
  .cp-mode {
    display: flex;
    gap: 16px;
    font-size: 0.82rem;
    color: var(--text-secondary);
  }
  .cp-mode-opt {
    display: flex;
    align-items: center;
    gap: 5px;
    cursor: pointer;
  }
  .cp-mode-opt input {
    accent-color: var(--accent);
    cursor: pointer;
  }
  .cp-error {
    font-size: 0.75rem;
    color: var(--danger);
    margin: 0;
  }
  .cp-btns {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }
  .cp-btn-primary {
    background: var(--accent);
    color: var(--bg-base);
    border: none;
    border-radius: 4px;
    padding: 6px 16px;
    font-size: 0.85rem;
    cursor: pointer;
    font-weight: 500;
    transition: opacity 0.12s;
  }
  .cp-btn-primary:disabled {
    opacity: 0.45;
    cursor: default;
  }
  .cp-btn-ghost {
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    padding: 6px 14px;
    font-size: 0.85rem;
    cursor: pointer;
    transition:
      background 0.1s,
      color 0.1s;
  }
  .cp-btn-ghost:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }

  /* ── Blink + Suggest toolbar buttons ─────────────────────────────────────── */
  .blink-btn {
    color: var(--accent);
    border-color: var(--accent);
  }
  .blink-btn:disabled {
    color: var(--text-secondary);
    border-color: var(--border);
    opacity: 0.5;
    cursor: not-allowed;
  }
  .suggest-btn {
    color: var(--accent);
  }

  /* ── Suggest rejects modal ─────────────────────────────────────────────── */
  .suggest-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 500;
  }
  .suggest-modal {
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    width: min(88vw, 680px);
    max-height: 80vh;
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.7);
    overflow: hidden;
  }
  .suggest-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .suggest-title {
    font-size: 0.88rem;
    font-weight: 600;
    color: var(--text-primary);
  }
  .suggest-close {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: 1rem;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
  }
  .suggest-close:hover {
    background: var(--bg-row-hover);
  }
  .suggest-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .suggest-status {
    color: var(--text-secondary);
    font-size: 0.85rem;
  }
  .suggest-desc {
    font-size: 0.8rem;
    color: var(--text-secondary);
    margin: 0;
  }
  .suggest-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .suggest-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.82rem;
    transition: background 0.1s;
  }
  .suggest-row:hover {
    background: var(--bg-row-hover);
  }
  .suggest-row.deselected {
    opacity: 0.45;
  }
  .suggest-row input {
    accent-color: var(--accent);
    flex-shrink: 0;
  }
  .suggest-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: "Consolas", monospace;
    color: var(--text-primary);
  }
  .suggest-obj {
    width: 90px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-secondary);
  }
  .suggest-fwhm {
    width: 120px;
    text-align: right;
    color: var(--danger);
    font-size: 0.78rem;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .suggest-footer {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 14px;
    border-top: 1px solid var(--border);
    flex-shrink: 0;
  }
  .suggest-sel-count {
    font-size: 0.78rem;
    color: var(--text-secondary);
    margin-right: auto;
  }
</style>
