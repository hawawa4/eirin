<script lang="ts">
  import { onMount } from "svelte";
  import {
    GetLibraryFrames,
    SetFrameType,
    RejectFile,
    UnrejectFile,
    HardDeleteFile,
    OpenWithSiril,
    AnalyzeFrames,
    CancelAnalysis,
  } from "../../wailsjs/go/app/App.js";
  import { EventsOn } from "../../wailsjs/runtime/runtime.js";
  import type { app } from "../../wailsjs/go/models";
  import type { AnalysisProgress, ColumnDef, CtxEntry, CtxMenuState, FrameType, LibraryGroupBy } from "../lib/types";
  import { getLibraryCellValue } from "../lib/utils";
  import ContextMenu from "./ContextMenu.svelte";
  import HardDeleteModal from "./HardDeleteModal.svelte";

  interface Props {
    rootFolder: string;
    columns: ColumnDef[];
    selectedNasPath: string | null;
    sirilAvailable: boolean;
    onfileclick: (frame: app.LibraryFrame) => void;
    onsavecolumns: () => void;
    onframesreloaded?: (frames: app.LibraryFrame[]) => void;
  }

  let { rootFolder, columns, selectedNasPath, sirilAvailable, onfileclick, onsavecolumns, onframesreloaded }: Props = $props();

  // ── Data ─────────────────────────────────────────────────────────────────
  let frames = $state<app.LibraryFrame[]>([]);
  let loading = $state(false);
  let error = $state("");

  // ── Toolbar state ─────────────────────────────────────────────────────────
  let showRejected = $state(false);
  let groupBy = $state<LibraryGroupBy>("object");
  let typeFilter = $state<FrameType | "all">("all");
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

  function sortValue(f: app.LibraryFrame, col: string): number | string {
    switch (col) {
      case "fwhm":       return f.qualityAnalyzed ? f.fwhm       : Infinity;
      case "starCount":  return f.qualityAnalyzed ? -f.starCount  : Infinity; // more = better
      case "background": return f.qualityAnalyzed ? f.background  : Infinity;
      case "noise":      return f.qualityAnalyzed ? f.noise       : Infinity;
      case "snr":        return f.qualityAnalyzed ? -f.snr        : Infinity; // higher = better
      case "expTime":    return -f.expTime;
      case "dateObs":   return f.dateObs;
      case "gain":      return f.gain;
      case "size":      return -f.fileSize;
      default:          return String((f as unknown as Record<string, unknown>)[col] ?? "");
    }
  }

  // ── Quality filters ───────────────────────────────────────────────────────
  let fwhmMax = $state(0);   // 0 = disabled
  let starsMin = $state(0);  // 0 = disabled
  let hideUnanalyzed = $state(false);

  // ── Column drag ───────────────────────────────────────────────────────────
  let dragSourceId = "";
  let dragOverIndex = $state(-1);

  // ── Context menu / delete modal ───────────────────────────────────────────
  let ctxMenu = $state<CtxMenuState | null>(null);
  let confirmDel = $state<{ path: string; name: string } | null>(null);

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
    const lightPaths = group.frames
      .filter((f) => f.frameType === "light")
      .map((f) => f.nasPath);
    if (!lightPaths.length) return;
    analyzingGroup = group.key;
    analysisProgress = null;
    try {
      await AnalyzeFrames(lightPaths);
    } finally {
      analyzingGroup = null;
      analysisProgress = null;
      reload();
    }
  }

  // ── Group collapse — empty set = all collapsed (default) ─────────────────
  let expandedGroups = $state<Set<string>>(new Set());

  function toggleGroup(key: string) {
    const next = new Set(expandedGroups);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    expandedGroups = next;
  }

  function expandAll() {
    expandedGroups = new Set(groups.map((g) => g.key));
  }

  function collapseAll() {
    expandedGroups = new Set();
  }

  const GROUP_BY_OPTIONS: { value: LibraryGroupBy; label: string }[] = [
    { value: "object", label: "Object" },
    { value: "date", label: "Date" },
    { value: "filter", label: "Filter" },
    { value: "frameType", label: "Type" },
  ];

  const FRAME_TYPE_META: Record<
    string,
    { label: string; short: string; color: string; bg: string }
  > = {
    light: { label: "Light", short: "LIGHT", color: "#60a5fa", bg: "#1e3a5f" },
    dark: { label: "Dark", short: "DARK", color: "#94a3b8", bg: "#1e2a3a" },
    flat: { label: "Flat", short: "FLAT", color: "#fbbf24", bg: "#3d2a00" },
    bias: { label: "Bias", short: "BIAS", color: "#a78bfa", bg: "#2d1f4a" },
    stacked: { label: "Stacked", short: "STACK", color: "#34d399", bg: "#0d3a2a" },
    processed: { label: "Processed", short: "PROC", color: "#f59e0b", bg: "#3d2d00" },
  };

  onMount(() => {
    reload();
  });

  async function reload() {
    if (!rootFolder || loading) return;
    loading = true;
    error = "";
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
      if (typeFilter !== "all" && f.frameType !== typeFilter) return false;
      if (hideUnanalyzed && !f.qualityAnalyzed) return false;
      if (fwhmMax > 0 && f.qualityAnalyzed && f.fwhm > fwhmMax) return false;
      if (starsMin > 0 && f.qualityAnalyzed && f.starCount < starsMin) return false;
      if (!search) return true;
      const q = search.toLowerCase();
      return (
        f.fileName.toLowerCase().includes(q) ||
        f.object.toLowerCase().includes(q) ||
        f.filter.toLowerCase().includes(q)
      );
    }),
  );

  let sorted = $derived(
    sortCol
      ? [...filtered].sort((a, b) => {
          const av = sortValue(a, sortCol!);
          const bv = sortValue(b, sortCol!);
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
    const map = new Map<string, app.LibraryFrame[]>();
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

  function lightPaths(group: LibGroup): string[] {
    return group.frames.filter((f) => f.frameType === "light").map((f) => f.nasPath);
  }

  function groupTypeBreakdown(frames: app.LibraryFrame[]) {
    const counts = new Map<string, number>();
    for (const f of frames) counts.set(f.frameType, (counts.get(f.frameType) ?? 0) + 1);
    return [...counts.entries()]
      .sort((a, b) => {
        const ai = TYPE_ORDER.indexOf(a[0]);
        const bi = TYPE_ORDER.indexOf(b[0]);
        return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
      })
      .map(([type, count]) => ({ type, count, meta: frameTypeMeta(type) }));
  }

  // ── Column resize ─────────────────────────────────────────────────────────
  function startColResize(e: MouseEvent, colId: string) {
    e.preventDefault();
    e.stopPropagation();
    const startX = e.clientX;
    const col = columns.find((c) => c.id === colId)!;
    const startWidth = col.width;

    function onMove(ev: MouseEvent) {
      col.width = Math.max(48, startWidth + ev.clientX - startX);
    }
    function onUp() {
      onsavecolumns();
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    }
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }

  // ── Column drag-reorder ───────────────────────────────────────────────────
  function onColDragStart(e: DragEvent, visIdx: number) {
    dragSourceId = visibleColumns[visIdx].id;
    e.dataTransfer!.effectAllowed = "move";
  }

  function onColDragOver(e: DragEvent, visIdx: number) {
    e.preventDefault();
    e.dataTransfer!.dropEffect = "move";
    dragOverIndex = visIdx;
  }

  function onColDrop(e: DragEvent, targetVisIdx: number) {
    e.preventDefault();
    dragOverIndex = -1;
    if (!dragSourceId) return;
    const srcVisIdx = visibleColumns.findIndex((c) => c.id === dragSourceId);
    dragSourceId = "";
    if (srcVisIdx === -1 || srcVisIdx === targetVisIdx) return;
    const newVis = [...visibleColumns];
    const [moved] = newVis.splice(srcVisIdx, 1);
    newVis.splice(targetVisIdx, 0, moved);
    let order = 0;
    for (const col of newVis) columns.find((c) => c.id === col.id)!.order = order++;
    for (const col of columns.filter((c) => !c.visible))
      columns.find((c) => c.id === col.id)!.order = order++;
    onsavecolumns();
  }

  function onColDragEnd() {
    dragSourceId = "";
    dragOverIndex = -1;
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

  // ── Frame type change ─────────────────────────────────────────────────────
  async function changeFrameType(nasPath: string, newType: string, e: Event) {
    e.stopPropagation();
    await SetFrameType(nasPath, newType);
    frames = frames.map((f) => (f.nasPath === nasPath ? { ...f, frameType: newType } : f));
  }

  // ── Reject / restore / hard delete ───────────────────────────────────────
  function openCtxMenu(e: MouseEvent, frame: app.LibraryFrame) {
    e.preventDefault();
    ctxMenu = {
      x: e.clientX,
      y: e.clientY,
      entry: { path: frame.nasPath, name: frame.fileName, isRejected: frame.isRejected },
      sirilAvailable,
    };
  }

  async function onCtxOpenWithSiril(entry: CtxEntry) {
    ctxMenu = null;
    await OpenWithSiril(entry.path);
  }

  async function onCtxReject(entry: CtxEntry) {
    ctxMenu = null;
    await RejectFile(entry.path);
    frames = frames.map((f) => (f.nasPath === entry.path ? { ...f, isRejected: true } : f));
  }

  async function onCtxRestore(entry: CtxEntry) {
    ctxMenu = null;
    await UnrejectFile(entry.path);
    frames = frames.map((f) => (f.nasPath === entry.path ? { ...f, isRejected: false } : f));
  }

  function onCtxHardDelete(entry: CtxEntry) {
    ctxMenu = null;
    confirmDel = { path: entry.path, name: entry.name };
  }

  async function doHardDelete() {
    if (!confirmDel) return;
    const { path } = confirmDel;
    confirmDel = null;
    await HardDeleteFile(path);
    frames = frames.filter((f) => f.nasPath !== path);
  }
</script>

<!-- ── Toolbar ───────────────────────────────────────────────────────────── -->
<div class="toolbar">
  <div class="toolbar-left">
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
          {#each GROUP_BY_OPTIONS as opt}
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
  </div>

  <div class="toolbar-right">
    <button class="tool-btn" onclick={expandAll} title="Expand all groups">⊞</button>
    <button class="tool-btn" onclick={collapseAll} title="Collapse all groups">⊟</button>
    {#if sortCol}
      <button class="tool-btn sort-clear" onclick={() => { sortCol = null; }} title="Clear sort">
        ✕ sort
      </button>
    {/if}
    <select class="type-filter" bind:value={typeFilter}>
      <option value="all">All types</option>
      {#each Object.entries(FRAME_TYPE_META) as [val, meta]}
        <option value={val}>{meta.label}</option>
      {/each}
    </select>
    <div class="quality-filters" title="Quality filters (only apply to analyzed frames)">
      <span class="qf-label">FWHM≤</span>
      <input class="qf-input" type="number" min="0" step="0.1" placeholder="—"
        bind:value={fwhmMax} title="Hide frames with FWHM above this value (0 = off)" />
      <span class="qf-label">Stars≥</span>
      <input class="qf-input" type="number" min="0" step="1" placeholder="—"
        bind:value={starsMin} title="Hide frames with fewer stars (0 = off)" />
      <label class="qf-check" title="Hide frames that have not been analyzed">
        <input type="checkbox" bind:checked={hideUnanalyzed} />
        <span>Analyzed</span>
      </label>
    </div>
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
</div>

<!-- ── Table ──────────────────────────────────────────────────────────────── -->
{#if loading}
  <div class="status-row">Loading library…</div>
{:else if error}
  <div class="status-row error">{error}</div>
{:else if groups.length === 0}
  <div class="status-row">
    {#if showRejected}
      No rejected frames.
    {:else if frames.filter((f) => !f.isRejected).length === 0}
      No indexed frames found. Run Build Index first.
    {:else}
      No frames match the current filter.
    {/if}
  </div>
{:else}
  <div class="table-scroll-wrapper">
    <table class="lib-table" style="width: {Math.max(totalColWidth, 100)}px; min-width: 100%">
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
              class:sorted={sortCol === col.id}
              draggable={col.id !== "frameType" && col.id !== "name"}
              onclick={() => toggleSort(col.id)}
              ondragstart={(e) => onColDragStart(e, i)}
              ondragover={(e) => onColDragOver(e, i)}
              ondrop={(e) => onColDrop(e, i)}
              ondragend={onColDragEnd}
              ondragleave={() => {
                if (dragOverIndex === i) dragOverIndex = -1;
              }}
            >
              <span class="th-text">{col.label}</span>
              {#if sortCol === col.id}
                <span class="sort-indicator">{sortDir === "asc" ? "▲" : "▼"}</span>
              {/if}
              <span
                class="resize-handle"
                onmousedown={(e) => { startColResize(e, col.id); }}
                role="separator"
                aria-label="Resize column"
              ></span>
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each groups as group (group.key)}
          <!-- Group header row -->
          <tr class="group-header-row" onclick={() => toggleGroup(group.key)}>
            <td colspan={visibleColumns.length}>
              <div class="group-hdr-inner">
              <span class="group-chevron">{expandedGroups.has(group.key) ? "▼" : "▶"}</span>
              <span class="group-label">{group.label}</span>
              <span class="group-count"
                >{group.frames.length} frame{group.frames.length !== 1 ? "s" : ""}</span
              >
              <span class="group-type-breakdown">
                {#each groupTypeBreakdown(group.frames) as { count, meta }}
                  <span class="group-type-badge" style="color:{meta.color};background:{meta.bg}">
                    {meta.short} {count}
                  </span>
                {/each}
              </span>
              {#if group.frames.some((f) => f.qualityAnalyzed)}
                <span class="quality-dot" title="Quality data available">✦</span>
              {/if}
              <span class="group-spacer"></span>
              {#if sirilAvailable && group.frames.some((f) => f.frameType === "light")}
                {#if analyzingGroup === group.key}
                  <span class="analysis-status">
                    ⟳ {analysisProgress?.done ?? 0}/{analysisProgress?.total ?? lightPaths(group).length}
                    {#if analysisProgress?.current}· {analysisProgress.current}{/if}
                  </span>
                  <button
                    class="btn-cancel-analysis"
                    onclick={(e) => { e.stopPropagation(); CancelAnalysis(); }}
                    title="Cancel analysis"
                  >✕</button>
                {:else}
                  <button
                    class="btn-analyze"
                    onclick={(e) => { e.stopPropagation(); analyzeGroup(group); }}
                    disabled={analyzingGroup !== null}
                    title="Analyze light frames with Siril (findstar)"
                  >✦ Analyze</button>
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
              onclick={() => onfileclick(frame)}
              oncontextmenu={(e) => openCtxMenu(e, frame)}
            >
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
                        changeFrameType(
                          frame.nasPath,
                          (e.target as HTMLSelectElement).value,
                          e,
                        )}
                    >
                      {#each Object.entries(FRAME_TYPE_META) as [val, m]}
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
  />
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

<style>
  /* ── Toolbar ─────────────────────────────────────────────────────────────── */

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 5px 8px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .toolbar-left,
  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 6px;
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
    transition: background 0.12s, color 0.12s;
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

  .type-filter {
    font-size: 0.78rem;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    padding: 2px 6px;
    cursor: pointer;
    max-width: 100px;
  }

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

  .quality-filters {
    display: flex;
    align-items: center;
    gap: 4px;
    border-left: 1px solid var(--border);
    padding-left: 8px;
    flex-shrink: 0;
  }

  .qf-label {
    font-size: 0.72rem;
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .qf-input {
    width: 52px;
    font-size: 0.78rem;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-primary);
    padding: 2px 5px;
    outline: none;
  }
  .qf-input:focus { border-color: var(--accent); }

  .qf-check {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 0.72rem;
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
    white-space: nowrap;
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
    position: sticky;
    top: 0;
    z-index: 1;
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
    position: relative;
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
    transition: background 0.12s, color 0.12s;
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
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
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
    border: none;
    border-radius: 3px;
    font-size: 0.68rem;
    font-weight: 600;
    font-family: "Consolas", "Fira Code", monospace;
    padding: 2px 5px;
    cursor: pointer;
    width: 100%;
    text-align: center;
    outline: none;
  }

  .type-select option {
    background: var(--bg-panel);
    color: var(--text-primary);
    font-weight: normal;
  }
</style>
