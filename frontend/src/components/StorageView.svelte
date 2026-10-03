<script lang="ts">
  import type * as app from "$models/app";
  import { GetStorageStats, GetFrameTypeSummary } from "$app";
  import { isTypingTarget, isModalOpen } from "../lib/keys";
  import { fmtFrames } from "../lib/storage/format";
  import { formatBytes } from "../lib/utils";
  import Treemap from "./storage/Treemap.svelte";

  interface Props {
    rootPath: string;
    /** True while this view's tab is visible. */
    active?: boolean;
    /** Request a library (re)scan/index. */
    onscan?: () => void;
  }

  let { rootPath, active = true, onscan }: Props = $props();

  // ── Data ──────────────────────────────────────────────────────────────────
  let root = $state<app.StorageNode | null>(null);
  let typeSummary = $state<app.StorageNode[]>([]);
  let loading = $state(false);
  let error = $state("");
  let reloadKey = $state(0);
  let requestId = 0;

  /** Drill-down path: labels from the root's children downwards. */
  let path = $state<string[]>([]);
  let hovered = $state<string | null>(null);

  $effect(() => {
    const rp = rootPath;
    void reloadKey;
    if (!rp) {
      root = null;
      typeSummary = [];
      error = "";
      loading = false;
      return;
    }
    const id = ++requestId;
    loading = true;
    error = "";
    Promise.all([GetStorageStats(rp), GetFrameTypeSummary(rp)])
      .then(([r, t]) => {
        if (id !== requestId) return;
        root = r;
        typeSummary = t ?? [];
        path = validPrefix(r, path);
      })
      .catch((e) => {
        if (id !== requestId) return;
        error = String(e);
      })
      .finally(() => {
        if (id === requestId) loading = false;
      });
  });

  /** Longest prefix of `p` that still exists in `tree` (after a refresh). */
  function validPrefix(tree: app.StorageNode | null, p: string[]): string[] {
    const out: string[] = [];
    let node = tree;
    for (const label of p) {
      const next = node?.children?.find((c) => c.label === label);
      if (!next?.children?.length) break;
      out.push(label);
      node = next;
    }
    return out;
  }

  let current = $derived.by(() => {
    let node = root;
    for (const label of path) node = node?.children?.find((c) => c.label === label) ?? node;
    return node;
  });
  let levelNodes = $derived(
    (current?.children ?? []).slice().sort((a, b) => b.totalBytes - a.totalBytes),
  );
  let levelTotal = $derived(levelNodes.reduce((s, n) => s + n.totalBytes, 0));
  let hasData = $derived(!!root && (root.children?.length ?? 0) > 0);

  function drill(node: app.StorageNode) {
    if (!node.children?.length) return;
    hovered = null;
    path = [...path, node.label];
  }

  function goTo(depth: number) {
    hovered = null;
    path = path.slice(0, depth);
  }

  function onKeydown(e: KeyboardEvent) {
    if (active === false || path.length === 0) return;
    if (e.key !== "Backspace" && e.key !== "Escape") return;
    if (isTypingTarget(e) || isModalOpen()) return;
    e.preventDefault();
    goTo(path.length - 1);
  }

  function typeLabel(t: string): string {
    return t ? t.charAt(0).toUpperCase() + t.slice(1) : "Unknown";
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="storage-view">
  <div class="storage-header">
    <h2 class="storage-title">Storage</h2>
    {#if root && hasData}
      <span class="storage-total"
        >{formatBytes(root.totalBytes)} · {fmtFrames(root.frameCount)}</span
      >
    {/if}
    <span class="spacer"></span>
    {#if loading && root}
      <span class="refreshing" role="status">Refreshing…</span>
    {/if}
    {#if rootPath}
      <button class="btn-secondary" disabled={loading} onclick={() => reloadKey++}>Refresh</button>
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
  {:else if loading && !root}
    <div class="status">Loading storage data…</div>
  {:else if error}
    <div class="status error" role="alert">
      Couldn't load storage data: {error}
      <button class="btn-secondary" onclick={() => reloadKey++}>Retry</button>
    </div>
  {:else if !hasData}
    <div class="empty">
      <p class="empty-title">No indexed frames yet</p>
      <p class="empty-sub">
        Eirin hasn't indexed any frames in <code>{rootPath}</code>. Scan the library to see how
        storage is used.
      </p>
      {#if onscan}<button class="btn-primary" onclick={onscan}>Scan library</button>{/if}
    </div>
  {:else if root}
    {#if typeSummary.length > 0 && path.length === 0}
      <div class="type-bar">
        {#each typeSummary as t (t.label)}
          <div class="type-chip">
            <span class="type-label">{typeLabel(t.label)}</span>
            <span class="type-size">{formatBytes(t.totalBytes)}</span>
            <span class="type-count">{fmtFrames(t.frameCount)}</span>
          </div>
        {/each}
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
        <span class="crumb-hint">Backspace or Esc to go up</span>
      {/if}
    </nav>

    <Treemap nodes={levelNodes} {hovered} onhover={(l) => (hovered = l)} ondrill={drill} />

    <ul class="object-list">
      {#each levelNodes as node (node.label)}
        {@const pct = levelTotal > 0 ? (node.totalBytes / levelTotal) * 100 : 0}
        <li
          class="object-row"
          class:hovered={hovered === node.label}
          onmouseenter={() => (hovered = node.label)}
          onmouseleave={() => (hovered = null)}
        >
          {#if node.children?.length}
            <button class="obj-label obj-link" onclick={() => drill(node)} title="Show {node.label}"
              >{node.label}</button
            >
          {:else}
            <span class="obj-label">{node.label}</span>
          {/if}
          <span class="obj-count">{fmtFrames(node.frameCount)}</span>
          <div class="obj-bar-wrap" title="{pct.toFixed(1)}% of this level">
            <div class="obj-bar" style:width="{pct}%"></div>
          </div>
          <span class="obj-size">{formatBytes(node.totalBytes)}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>

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
  .storage-header .btn-secondary {
    margin-right: 0;
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

  /* ── Type summary ───────────────────────────────────────────────────── */
  .type-bar {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    flex-shrink: 0;
  }
  .type-chip {
    display: flex;
    align-items: baseline;
    gap: 8px;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 4px 10px;
    font-size: var(--fs-sm);
  }
  .type-label {
    color: var(--text-primary);
    font-weight: 600;
  }
  .type-size {
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  .type-count {
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
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
    padding: 5px 8px;
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
  .obj-size {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    width: 76px;
    text-align: right;
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }
</style>
