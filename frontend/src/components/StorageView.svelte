<script lang="ts">
  import { untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import type * as app from "$models/app";
  import {
    GetLibraryFrames,
    BatchRejectFiles,
    BatchUnrejectFiles,
    BatchHardDeleteFiles,
  } from "$app";
  import { Events } from "@wailsio/runtime";
  import { isTypingTarget, isModalOpen } from "../lib/keys";
  import { fmtFrames } from "../lib/storage/format";
  import {
    buildStorage,
    commonFolder,
    nodeAt,
    validPrefix,
    type StorageGroup,
  } from "../lib/storage/tree";
  import { formatBytes, plural } from "../lib/utils";
  import { FRAME_TYPE_META, type FrameType } from "../lib/types";
  import type { LibraryFocusFilter } from "../lib/library/filters";
  import { forgetPreview } from "../lib/library/previewCache";
  import { attempt, toast } from "../lib/toast.svelte";
  import { openFolder } from "../lib/shell/fileActions";
  import { copyPath } from "../lib/clipboard";
  import Treemap from "./storage/Treemap.svelte";
  import ActionMenu, { type ActionMenuItem } from "./ActionMenu.svelte";
  import HardDeleteModal from "./HardDeleteModal.svelte";

  interface Props {
    rootPath: string;
    /** True while this view's tab is visible. */
    active?: boolean;
    /** Request a library (re)scan/index. */
    onscan?: () => void;
    /** Open the Library filtered to a node's frames. */
    onshowinlibrary?: (f: LibraryFocusFilter) => void;
    /** Open the Library with these frames checked (for nodes a filter can't express). */
    onselectinlibrary?: (paths: string[]) => void;
  }

  let { rootPath, active = true, onscan, onshowinlibrary, onselectinlibrary }: Props = $props();

  // ── Data ──────────────────────────────────────────────────────────────────
  let frames = $state.raw<app.LibraryFrame[]>([]);
  let loaded = $state(false);
  let loading = $state(false);
  let error = $state("");
  let requestId = 0;

  async function reload() {
    const rp = rootPath;
    if (!rp) return;
    const id = ++requestId;
    loading = true;
    try {
      const res = (await GetLibraryFrames(rp)) ?? [];
      if (id !== requestId) return;
      frames = res;
      loaded = true;
      error = "";
    } catch (e) {
      if (id !== requestId) return;
      if (loaded) toast.error(`Couldn't refresh storage data: ${String(e)}`);
      else error = String(e);
    } finally {
      if (id === requestId) loading = false;
    }
  }

  $effect(() => {
    void rootPath;
    untrack(() => {
      frames = [];
      loaded = false;
      error = "";
      path = [];
      reload();
    });
  });

  // Picks up scans, imports, rejects and deletes made in other views. Only
  // reloads while visible; switching back to the tab catches up.
  let stale = false;
  $effect(() => {
    return Events.On("library:updated", () => {
      if (active) reload();
      else stale = true;
    });
  });
  $effect(() => {
    if (active && stale) {
      stale = false;
      untrack(reload);
    }
  });

  // ── Lens: frame types / rejected only ─────────────────────────────────────
  const types = new SvelteSet<string>();
  let rejectedOnly = $state(false);

  function toggleType(t: string, additive: boolean) {
    if (!additive) {
      const only = types.size === 1 && types.has(t);
      types.clear();
      if (!only) types.add(t);
      return;
    }
    if (types.has(t)) types.delete(t);
    else types.add(t);
  }

  function clearLens() {
    types.clear();
    rejectedOnly = false;
  }

  let summary = $derived(buildStorage(frames, types, rejectedOnly));
  let total = $derived(frames.reduce((s, f) => s + f.fileSize, 0));
  let lensActive = $derived(types.size > 0 || rejectedOnly);

  // ── Drill-down ────────────────────────────────────────────────────────────
  /** Labels from the root's children downwards. */
  let path = $state<string[]>([]);
  let hovered = $state<string | null>(null);

  // A refresh or lens change can remove the node we're in; climb to what's left.
  $effect(() => {
    const valid = validPrefix(
      summary.root,
      untrack(() => path),
    );
    if (valid.length !== untrack(() => path.length)) path = valid;
  });

  let current = $derived(nodeAt(summary.root, path) ?? summary.root);
  let levelNodes = $derived(current.children ?? []);
  let levelTotal = $derived(levelNodes.reduce((s, n) => s + n.totalBytes, 0));

  function drill(node: StorageGroup) {
    if (!node.children?.length) return;
    hovered = null;
    path = [...path, node.label];
  }

  function goTo(depth: number) {
    hovered = null;
    path = path.slice(0, depth);
  }

  function onKeydown(e: KeyboardEvent) {
    if (active === false || path.length === 0 || menu || confirmDel) return;
    if (e.key !== "Backspace" && e.key !== "Escape") return;
    if (isTypingTarget(e) || isModalOpen()) return;
    e.preventDefault();
    goTo(path.length - 1);
  }

  function typeLabel(t: string): string {
    return FRAME_TYPE_META[t]?.label ?? (t ? t.charAt(0).toUpperCase() + t.slice(1) : "Unknown");
  }

  /** "M 31", "M 31 · 2026-03-25", "All objects". */
  function nodeTitle(node: StorageGroup): string {
    if (node === summary.root) return "All objects";
    return node.date !== undefined
      ? `${node.object || "Unknown object"} · ${node.label}`
      : node.label;
  }

  // ── Actions ───────────────────────────────────────────────────────────────
  function showInLibrary(node: StorageGroup, rejected = rejectedOnly) {
    const isRoot = node === summary.root;
    if (!isRoot && !node.object) {
      // The Library's filters can't express "no object"; check the frames instead.
      onselectinlibrary?.(rejected ? node.rejectedPaths : node.paths);
      return;
    }
    onshowinlibrary?.({
      object: isRoot ? undefined : node.object,
      date: node.date || undefined,
      types: [...types] as FrameType[],
      rejected,
    });
  }

  async function rejectNode(node: StorageGroup) {
    const rejected = new Set(node.rejectedPaths);
    const list = node.paths.filter((p) => !rejected.has(p));
    if (list.length === 0) return;
    const ok = await attempt(
      async () => {
        await BatchRejectFiles(list);
        return true;
      },
      `Couldn't reject ${plural(list.length, "frame")}`,
    );
    await reload();
    if (!ok) return;
    toast.success(`Rejected ${plural(list.length, "frame")} in ${nodeTitle(node)}`, {
      action: {
        label: "Undo",
        run: async () => {
          await attempt(() => BatchUnrejectFiles(list), "Couldn't restore the frames");
          await reload();
        },
      },
    });
  }

  let confirmDel = $state<{ paths: string[]; name: string; count: number; bytes: number } | null>(
    null,
  );

  function askDelete(node: StorageGroup, onlyRejected: boolean) {
    const paths = onlyRejected ? node.rejectedPaths : node.paths;
    if (paths.length === 0) return;
    const what = onlyRejected ? "rejected frames" : "frames";
    confirmDel = {
      paths,
      name: `${nodeTitle(node)} — ${paths.length.toLocaleString()} ${paths.length === 1 ? what.slice(0, -1) : what}`,
      count: paths.length,
      bytes: onlyRejected ? node.rejectedBytes : node.totalBytes,
    };
  }

  async function confirmDelete() {
    const target = confirmDel;
    confirmDel = null;
    if (!target) return;
    const ok = await attempt(
      async () => {
        await BatchHardDeleteFiles(target.paths);
        return true;
      },
      `Couldn't delete ${plural(target.paths.length, "file")}`,
    );
    for (const p of target.paths) forgetPreview(p);
    await reload();
    if (ok) {
      toast.success(
        `Deleted ${plural(target.paths.length, "file")} · ${formatBytes(target.bytes)} freed`,
      );
    }
  }

  let menu = $state<{ x: number; y: number; items: ActionMenuItem[] } | null>(null);

  function openMenu(node: StorageGroup, x: number, y: number) {
    const live = node.frameCount - node.rejectedPaths.length;
    const folder = commonFolder(node.paths);
    const items: ActionMenuItem[] = [
      { heading: `${nodeTitle(node)} · ${formatBytes(node.totalBytes)}` },
    ];
    if (onshowinlibrary) {
      items.push({ label: "Show in Library", primary: true, run: () => showInLibrary(node) });
    }
    if (node.children?.length && node !== current) {
      items.push({ label: "Open nights", run: () => drill(node) });
    }
    if (folder) {
      items.push(
        { label: "Open folder", run: () => openFolder(folder) },
        { label: "Copy folder path", run: () => copyPath(folder) },
      );
    }
    items.push({ separator: true });
    if (!rejectedOnly) {
      items.push({
        label: live > 0 ? `Reject ${plural(live, "frame")}` : "Reject frames",
        disabledReason: live === 0 ? "Every frame here is already rejected" : undefined,
        run: () => rejectNode(node),
      });
    }
    if (node.rejectedPaths.length > 0) {
      items.push({
        label: `Delete ${plural(node.rejectedPaths.length, "rejected frame")} (${formatBytes(node.rejectedBytes)})…`,
        danger: true,
        run: () => askDelete(node, true),
      });
    }
    if (!rejectedOnly) {
      items.push({
        label: `Delete all ${plural(node.frameCount, "frame")} (${formatBytes(node.totalBytes)})…`,
        danger: true,
        run: () => askDelete(node, false),
      });
    }
    menu = { x, y, items };
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="storage-view">
  <div class="storage-header">
    <h2 class="storage-title">Storage</h2>
    {#if loaded && frames.length > 0}
      <span class="storage-total">{formatBytes(total)} · {fmtFrames(frames.length)}</span>
    {/if}
    <span class="spacer"></span>
    {#if loading && loaded}
      <span class="refreshing" role="status">Refreshing…</span>
    {/if}
    {#if rootPath}
      <button class="btn-secondary" disabled={loading} onclick={reload}>Refresh</button>
    {/if}
  </div>

  {#if !rootPath}
    <div class="empty">
      <p class="empty-title">No library folder selected</p>
      <p class="empty-sub">
        Choose a library folder in Settings, then scan it to see how storage is used.
      </p>
      {#if onscan}<button class="btn-primary" onclick={onscan}>Scan library</button>{/if}
    </div>
  {:else if loading && !loaded}
    <div class="status">Loading storage data…</div>
  {:else if error}
    <div class="status error" role="alert">
      Couldn't load storage data: {error}
      <button class="btn-secondary" onclick={reload}>Retry</button>
    </div>
  {:else if frames.length === 0}
    <div class="empty">
      <p class="empty-title">No indexed frames yet</p>
      <p class="empty-sub">
        Eirin hasn't indexed any frames in <code>{rootPath}</code>. Scan the library to see how
        storage is used.
      </p>
      {#if onscan}<button class="btn-primary" onclick={onscan}>Scan library</button>{/if}
    </div>
  {:else}
    <div class="lens" role="group" aria-label="Show only">
      {#each summary.byType as t (t.type)}
        {@const meta = FRAME_TYPE_META[t.type]}
        <button
          class="lens-chip"
          class:active={types.has(t.type)}
          style:--chip-color={meta?.color ?? "var(--text-secondary)"}
          aria-pressed={types.has(t.type)}
          onclick={(e) => toggleType(t.type, e.ctrlKey || e.metaKey || e.shiftKey)}
          title="Show only {typeLabel(t.type).toLowerCase()} frames (Ctrl+click to combine)"
        >
          <span class="lens-dot" aria-hidden="true"></span>
          <span class="lens-label">{typeLabel(t.type)}</span>
          <span class="lens-size">{formatBytes(t.bytes)}</span>
          <span class="lens-count">{t.count.toLocaleString()}</span>
        </button>
      {/each}
      {#if summary.rejectedCount > 0}
        <span class="lens-sep" aria-hidden="true"></span>
        <button
          class="lens-chip rejected"
          class:active={rejectedOnly}
          aria-pressed={rejectedOnly}
          onclick={() => (rejectedOnly = !rejectedOnly)}
          title="Show only rejected frames — the space you can free by deleting them"
        >
          <span class="lens-label">Rejected</span>
          <span class="lens-size">{formatBytes(summary.rejectedBytes)}</span>
          <span class="lens-count">{summary.rejectedCount.toLocaleString()}</span>
        </button>
      {/if}
      {#if lensActive}
        <button class="lens-clear" onclick={clearLens}>Show all</button>
      {/if}
    </div>

    {#if rejectedOnly && summary.root.frameCount > 0}
      <div class="reclaim" role="note">
        <span>
          <strong>{formatBytes(summary.root.totalBytes)}</strong> can be freed by deleting
          {plural(summary.root.frameCount, "rejected frame")}{types.size > 0
            ? " of these types"
            : ""}.
        </span>
        {#if onshowinlibrary}
          <button class="btn-secondary" onclick={() => showInLibrary(summary.root, true)}
            >Review in Library</button
          >
        {/if}
        <button class="btn-danger" onclick={() => askDelete(summary.root, true)}
          >Delete all rejected…</button
        >
      </div>
    {/if}

    <nav class="breadcrumb" aria-label="Storage level">
      {#if path.length === 0}
        <span class="crumb current" aria-current="page">All objects</span>
      {:else}
        <button class="crumb" onclick={() => goTo(0)}>All objects</button>
        {#each path as label, i (i)}
          <span class="crumb-sep" aria-hidden="true">›</span>
          {#if i === path.length - 1}
            <span class="crumb current" aria-current="page">{label}</span>
          {:else}
            <button class="crumb" onclick={() => goTo(i + 1)}>{label}</button>
          {/if}
        {/each}
      {/if}
      <span class="crumb-hint">
        {path.length > 0
          ? "Backspace to go up · "
          : "Click an object to see its nights · "}Right-click a tile or row for actions
      </span>
      {#if path.length > 0}
        <span class="spacer"></span>
        <button class="btn-ghost small" onclick={(e) => openMenu(current, e.clientX, e.clientY)}
          >Actions for {current.label} ▾</button
        >
      {/if}
    </nav>

    {#if levelNodes.length === 0}
      <div class="status">Nothing matches this filter.</div>
    {:else}
      <Treemap
        nodes={levelNodes}
        {hovered}
        onhover={(l) => (hovered = l)}
        ondrill={drill}
        onmenu={openMenu}
      />

      <ul class="object-list">
        {#each levelNodes as node (node.label)}
          {@const pct = levelTotal > 0 ? (node.totalBytes / levelTotal) * 100 : 0}
          {@const rejPct = levelTotal > 0 ? (node.rejectedBytes / levelTotal) * 100 : 0}
          <li
            class="object-row"
            class:hovered={hovered === node.label}
            onmouseenter={() => (hovered = node.label)}
            onmouseleave={() => (hovered = null)}
            oncontextmenu={(e) => {
              e.preventDefault();
              openMenu(node, e.clientX, e.clientY);
            }}
          >
            {#if node.children?.length}
              <button
                class="obj-label obj-link"
                onclick={() => drill(node)}
                title="Show the nights of {node.label}">{node.label}</button
              >
            {:else}
              <span class="obj-label">{node.label}</span>
            {/if}
            <span class="obj-count">{fmtFrames(node.frameCount)}</span>
            <div class="obj-bar-wrap" title="{pct.toFixed(1)}% of this level">
              <div class="obj-bar" style:width="{pct}%"></div>
              {#if rejPct > 0 && !rejectedOnly}
                <div
                  class="obj-bar-rej"
                  style:left="{pct - rejPct}%"
                  style:width="{rejPct}%"
                  title="{formatBytes(node.rejectedBytes)} rejected"
                ></div>
              {/if}
            </div>
            <span class="obj-size">{formatBytes(node.totalBytes)}</span>
            <button
              class="row-menu"
              aria-label="Actions for {node.label}"
              title="Actions"
              onclick={(e) => {
                const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
                openMenu(node, r.left, r.bottom + 2);
              }}>⋯</button
            >
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
</div>

{#if menu}
  <ActionMenu x={menu.x} y={menu.y} items={menu.items} onclose={() => (menu = null)} />
{/if}

{#if confirmDel}
  <HardDeleteModal
    target={confirmDel}
    onconfirm={confirmDelete}
    oncancel={() => (confirmDel = null)}
  />
{/if}

<style>
  .storage-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: auto;
    padding: 16px 20px;
    gap: 14px;
    min-height: 0;
  }

  .storage-header {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-shrink: 0;
  }
  .storage-title {
    font-size: var(--fs-lg);
    font-weight: 700;
    color: var(--text-primary);
    margin: 0;
  }
  .storage-total {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .spacer {
    flex: 1;
  }
  .refreshing {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }

  .status {
    color: var(--text-secondary);
    font-size: var(--fs-md);
    padding: 20px 0;
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .status.error {
    color: var(--danger);
  }

  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    text-align: center;
    padding: 32px;
  }
  .empty-title {
    margin: 0;
    font-size: var(--fs-lg);
    font-weight: 600;
    color: var(--text-primary);
  }
  .empty-sub {
    margin: 0 0 8px;
    max-width: 480px;
    font-size: var(--fs-md);
    color: var(--text-secondary);
    word-break: break-word;
  }
  .empty-sub code {
    font-family: monospace;
    color: var(--text-primary);
  }

  /* ── Lens chips ─────────────────────────────────────────────────────── */
  .lens {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    flex-shrink: 0;
  }
  .lens-chip {
    display: flex;
    align-items: baseline;
    gap: 8px;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 11px;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    cursor: pointer;
    transition:
      border-color 0.12s,
      background 0.12s;
  }
  .lens-chip:hover {
    border-color: var(--chip-color, var(--accent));
  }
  .lens-chip.active {
    border-color: var(--chip-color, var(--accent));
    background: color-mix(in srgb, var(--bg-panel) 75%, var(--chip-color, var(--accent)) 25%);
  }
  .lens-chip.rejected {
    --chip-color: var(--danger);
  }
  .lens-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--chip-color);
    align-self: center;
  }
  .lens-label {
    font-weight: 600;
  }
  .lens-size {
    font-variant-numeric: tabular-nums;
  }
  .lens-count {
    color: var(--text-secondary);
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
  }
  .lens-sep {
    width: 1px;
    align-self: stretch;
    background: var(--border);
    margin: 0 2px;
  }
  .lens-clear {
    background: none;
    border: none;
    color: var(--accent);
    font-size: var(--fs-sm);
    cursor: pointer;
    padding: 4px 6px;
  }
  .lens-clear:hover {
    text-decoration: underline;
  }

  .reclaim {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
    padding: 8px 12px;
    border: 1px solid color-mix(in srgb, var(--danger) 50%, var(--border));
    background: color-mix(in srgb, var(--bg-panel) 88%, var(--danger) 12%);
    border-radius: 6px;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    flex-shrink: 0;
  }
  .reclaim span {
    flex: 1;
    min-width: 200px;
  }

  /* ── Breadcrumb ─────────────────────────────────────────────────────── */
  .breadcrumb {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
    font-size: var(--fs-sm);
    flex-shrink: 0;
  }
  .crumb {
    background: none;
    border: none;
    padding: 2px 4px;
    font: inherit;
    color: var(--accent);
    cursor: pointer;
    border-radius: 3px;
  }
  .crumb:hover {
    background: var(--bg-row-hover);
  }
  .crumb.current {
    color: var(--text-primary);
    font-weight: 600;
    cursor: default;
  }
  .crumb-sep {
    color: var(--text-secondary);
  }
  .crumb-hint {
    margin-left: 8px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .btn-ghost.small {
    font-size: var(--fs-xs);
    padding: 3px 10px;
  }

  /* ── Object list (table view of the map) ────────────────────────────── */
  .object-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex-shrink: 0;
  }
  .object-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 3px 4px 3px 8px;
    border-radius: 4px;
    transition: background 0.1s;
  }
  .object-row.hovered {
    background: var(--bg-row-hover);
  }
  .obj-label {
    font-size: var(--fs-sm);
    color: var(--text-primary);
    width: 180px;
    flex-shrink: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    text-align: left;
  }
  .obj-link {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    font-size: var(--fs-sm);
    color: var(--accent);
    cursor: pointer;
  }
  .obj-link:hover {
    text-decoration: underline;
  }
  .obj-count {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    width: 90px;
    text-align: right;
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }
  .obj-bar-wrap {
    position: relative;
    flex: 1;
    height: 6px;
    background: var(--bg-row);
    border-radius: 3px;
    overflow: hidden;
  }
  .obj-bar {
    height: 100%;
    background: var(--accent);
    border-radius: 3px;
    transition: width 0.3s;
  }
  .obj-bar-rej {
    position: absolute;
    top: 0;
    bottom: 0;
    background: var(--danger);
  }
  .obj-size {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    width: 76px;
    text-align: right;
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }
  .row-menu {
    width: 26px;
    height: 24px;
    flex-shrink: 0;
    padding: 0;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.1s;
  }
  .object-row:hover .row-menu,
  .row-menu:focus-visible {
    opacity: 1;
  }
  .row-menu:hover {
    border-color: var(--border-accent);
    color: var(--text-primary);
  }
</style>
