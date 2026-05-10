<script lang="ts">
  import { onMount } from "svelte";
  import { GetLibraryFrames, SetFrameType } from "../../wailsjs/go/app/App.js";
  import type { app } from "../../wailsjs/go/models";
  import type { LibraryGroup, LibraryGroupBy, FrameType } from "../lib/types";
  import { formatSize } from "../lib/utils";

  interface Props {
    rootFolder: string;
    onfileclick: (nasPath: string) => void;
  }

  let { rootFolder, onfileclick }: Props = $props();

  let frames = $state<app.LibraryFrame[]>([]);
  let loading = $state(false);
  let error = $state("");
  let search = $state("");
  let groupBy = $state<LibraryGroupBy>("object");
  let expandedGroups = $state<Set<string>>(new Set());
  let typeFilter = $state<FrameType | "all">("all");

  const GROUP_BY_OPTIONS: { value: LibraryGroupBy; label: string }[] = [
    { value: "object", label: "Object" },
    { value: "date", label: "Date" },
    { value: "filter", label: "Filter" },
    { value: "frameType", label: "Frame Type" },
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
    if (!rootFolder) return;
    loading = true;
    error = "";
    try {
      frames = (await GetLibraryFrames(rootFolder)) ?? [];
    } catch (e) {
      error = String(e);
      frames = [];
    } finally {
      loading = false;
    }
  }

  // Expose reload so parent can trigger it after index completes
  export { reload };

  let filtered = $derived(
    frames.filter((f) => {
      if (typeFilter !== "all" && f.frameType !== typeFilter) return false;
      if (!search) return true;
      const q = search.toLowerCase();
      return (
        f.fileName.toLowerCase().includes(q) ||
        f.object.toLowerCase().includes(q) ||
        f.filter.toLowerCase().includes(q)
      );
    }),
  );

  let groups = $derived<LibraryGroup[]>(buildGroups(filtered, groupBy));

  function buildGroups(items: app.LibraryFrame[], by: LibraryGroupBy): LibraryGroup[] {
    const map = new Map<string, app.LibraryFrame[]>();
    for (const f of items) {
      const key = groupKey(f, by);
      const bucket = map.get(key);
      if (bucket) {
        bucket.push(f);
      } else {
        map.set(key, [f]);
      }
    }
    const result: LibraryGroup[] = [];
    for (const [key, groupFrames] of map) {
      result.push({ key, label: groupLabel(key, by, groupFrames), frames: groupFrames });
    }
    result.sort((a, b) => a.key.localeCompare(b.key));
    return result;
  }

  function groupKey(f: app.LibraryFrame, by: LibraryGroupBy): string {
    switch (by) {
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

  function groupLabel(key: string, by: LibraryGroupBy, items: app.LibraryFrame[]): string {
    if (by === "frameType") {
      return FRAME_TYPE_META[key]?.label ?? key;
    }
    return key;
  }

  function toggleGroup(key: string) {
    const next = new Set(expandedGroups);
    if (next.has(key)) {
      next.delete(key);
    } else {
      next.add(key);
    }
    expandedGroups = next;
  }

  function isExpanded(key: string): boolean {
    return expandedGroups.has(key);
  }

  function frameTypeMeta(type: string) {
    return FRAME_TYPE_META[type] ?? { label: type, short: type.toUpperCase(), color: "#9ca3af", bg: "#1f2937" };
  }

  function formatDate(dateObs: string): string {
    if (!dateObs) return "—";
    return dateObs.slice(0, 16).replace("T", " ");
  }

  function formatExp(expTime: number): string {
    if (!expTime) return "—";
    if (expTime >= 60) return `${(expTime / 60).toFixed(1)}m`;
    return `${expTime}s`;
  }

  async function changeFrameType(nasPath: string, newType: string) {
    await SetFrameType(nasPath, newType);
    frames = frames.map((f) => (f.nasPath === nasPath ? { ...f, frameType: newType } : f));
  }

  let totalCount = $derived(filtered.length);
  let lightCount = $derived(filtered.filter((f) => f.frameType === "light").length);
  let stackedCount = $derived(filtered.filter((f) => f.frameType === "stacked").length);
</script>

<div class="library">
  <!-- Toolbar -->
  <div class="toolbar">
    <div class="toolbar-left">
      <span class="label">Group by</span>
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
    <div class="toolbar-right">
      <select
        class="type-filter"
        bind:value={typeFilter}
      >
        <option value="all">All types</option>
        {#each Object.entries(FRAME_TYPE_META) as [val, meta]}
          <option value={val}>{meta.label}</option>
        {/each}
      </select>
      <input class="search" type="search" placeholder="Search…" bind:value={search} />
    </div>
  </div>

  <!-- Stats bar -->
  <div class="stats-bar">
    <span>{totalCount} frames</span>
    {#if lightCount > 0}<span class="stat-chip light">{lightCount} lights</span>{/if}
    {#if stackedCount > 0}<span class="stat-chip stacked">{stackedCount} stacked</span>{/if}
  </div>

  <!-- Content -->
  {#if loading}
    <div class="state-msg">Loading library…</div>
  {:else if error}
    <div class="state-msg error">{error}</div>
  {:else if groups.length === 0}
    <div class="state-msg">
      {frames.length === 0
        ? "No indexed frames found. Run Build Index first."
        : "No frames match the current filter."}
    </div>
  {:else}
    <div class="groups-list">
      {#each groups as group}
        {@const expanded = isExpanded(group.key)}
        <div class="group">
          <!-- Group header -->
          <button class="group-header" onclick={() => toggleGroup(group.key)}>
            <span class="group-chevron">{expanded ? "▼" : "▶"}</span>
            <span class="group-name">{group.label}</span>
            <span class="group-count">{group.frames.length} frame{group.frames.length !== 1 ? "s" : ""}</span>
            <!-- Type distribution pills -->
            <span class="group-types">
              {#each Object.entries( group.frames.reduce( (acc, f) => { acc[f.frameType] = (acc[f.frameType] ?? 0) + 1; return acc; }, {} as Record<string, number>, ), ) as [type, count]}
                {@const meta = frameTypeMeta(type)}
                <span class="type-pill" style="color:{meta.color};background:{meta.bg}">
                  {meta.short} {count}
                </span>
              {/each}
            </span>
          </button>

          <!-- Frame rows -->
          {#if expanded}
            <div class="frame-rows">
              {#each group.frames as frame}
                {@const meta = frameTypeMeta(frame.frameType)}
                <div
                  class="frame-row"
                  class:rejected={frame.isRejected}
                  role="button"
                  tabindex="0"
                  onclick={() => onfileclick(frame.nasPath)}
                  onkeydown={(e) => e.key === "Enter" && onfileclick(frame.nasPath)}
                >
                  <span
                    class="type-badge"
                    style="color:{meta.color};background:{meta.bg}"
                    title="Change frame type"
                    role="button"
                    tabindex="0"
                    onclick={(e) => e.stopPropagation()}
                    onkeydown={(e) => e.stopPropagation()}
                  >
                    <!-- Type change popover via select on the badge itself -->
                    <select
                      class="type-select"
                      value={frame.frameType}
                      style="color:{meta.color}"
                      onclick={(e) => e.stopPropagation()}
                      onchange={(e) => {
                        e.stopPropagation();
                        changeFrameType(frame.nasPath, (e.target as HTMLSelectElement).value);
                      }}
                    >
                      {#each Object.entries(FRAME_TYPE_META) as [val, m]}
                        <option value={val}>{m.short}</option>
                      {/each}
                    </select>
                  </span>

                  <span class="frame-name" title={frame.nasPath}>{frame.fileName}</span>
                  <span class="frame-cell filter">{frame.filter || "—"}</span>
                  <span class="frame-cell date">{formatDate(frame.dateObs)}</span>
                  <span class="frame-cell exp">{formatExp(frame.expTime)}</span>
                  <span class="frame-cell size">{formatSize(frame.fileSize, false)}</span>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .library {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  /* ── Toolbar ─────────────────────────────────────────────────────────────── */

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 8px 16px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .toolbar-left,
  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .label {
    font-size: 0.78rem;
    color: var(--text-secondary);
  }

  .segmented {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 4px;
    overflow: hidden;
  }

  .seg-btn {
    padding: 3px 10px;
    font-size: 0.78rem;
    background: transparent;
    border: none;
    color: var(--text-secondary);
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

  .type-filter {
    font-size: 0.78rem;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    padding: 3px 6px;
    cursor: pointer;
  }

  .search {
    font-size: 0.78rem;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    padding: 3px 8px;
    width: 160px;
    outline: none;
  }

  .search:focus {
    border-color: var(--accent-dim);
  }

  /* ── Stats bar ───────────────────────────────────────────────────────────── */

  .stats-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 16px;
    font-size: 0.75rem;
    color: var(--text-secondary);
    background: var(--bg-base);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .stat-chip {
    padding: 1px 7px;
    border-radius: 10px;
    font-size: 0.72rem;
    font-weight: 500;
  }

  .stat-chip.light {
    color: #60a5fa;
    background: #1e3a5f;
  }

  .stat-chip.stacked {
    color: #34d399;
    background: #0d3a2a;
  }

  /* ── State messages ──────────────────────────────────────────────────────── */

  .state-msg {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.875rem;
    color: var(--text-secondary);
  }

  .state-msg.error {
    color: var(--color-danger, #f87171);
  }

  /* ── Groups ──────────────────────────────────────────────────────────────── */

  .groups-list {
    flex: 1;
    overflow-y: auto;
  }

  .group {
    border-bottom: 1px solid var(--border);
  }

  .group-header {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    background: var(--bg-panel);
    border: none;
    cursor: pointer;
    text-align: left;
    color: var(--text-primary);
    font-size: 0.82rem;
    font-weight: 500;
    transition: background 0.12s;
  }

  .group-header:hover {
    background: var(--bg-row-hover);
  }

  .group-chevron {
    font-size: 0.65rem;
    color: var(--text-secondary);
    width: 10px;
    flex-shrink: 0;
  }

  .group-name {
    flex: 1;
    color: var(--text-primary);
  }

  .group-count {
    font-size: 0.75rem;
    color: var(--text-secondary);
  }

  .group-types {
    display: flex;
    gap: 4px;
  }

  .type-pill {
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 0.68rem;
    font-weight: 600;
    font-family: "Consolas", "Fira Code", monospace;
  }

  /* ── Frame rows ──────────────────────────────────────────────────────────── */

  .frame-rows {
    background: var(--bg-base);
  }

  .frame-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 16px 5px 32px;
    font-size: 0.8rem;
    cursor: pointer;
    border-top: 1px solid var(--border);
    color: var(--text-primary);
    transition: background 0.1s;
    outline: none;
  }

  .frame-row:hover {
    background: var(--bg-row-hover);
  }

  .frame-row.rejected {
    opacity: 0.45;
    text-decoration: line-through;
  }

  .type-badge {
    position: relative;
    flex-shrink: 0;
  }

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
    background: inherit;
    min-width: 44px;
    text-align: center;
    outline: none;
  }

  .type-select option {
    background: var(--bg-panel);
    color: var(--text-primary);
    font-weight: normal;
  }

  .frame-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: "Consolas", "Fira Code", monospace;
    font-size: 0.78rem;
  }

  .frame-cell {
    flex-shrink: 0;
    color: var(--text-secondary);
    font-size: 0.78rem;
    white-space: nowrap;
  }

  .frame-cell.filter {
    width: 70px;
    text-align: left;
  }

  .frame-cell.date {
    width: 130px;
    text-align: left;
  }

  .frame-cell.exp {
    width: 55px;
    text-align: right;
  }

  .frame-cell.size {
    width: 65px;
    text-align: right;
  }
</style>
