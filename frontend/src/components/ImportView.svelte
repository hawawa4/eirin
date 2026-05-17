<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import {
    SelectSourceFolder,
    ScanImportCandidates,
    StartImport,
  } from "../../wailsjs/go/app/App.js";
  import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime.js";
  import type { ImportCandidate, ImportProgress, ImportState } from "../lib/types";

  interface Props {
    rootFolder: string;
  }

  let { rootFolder }: Props = $props();

  let phase = $state<ImportState>("idle");
  let sourceFolder = $state("");
  let candidates = $state<ImportCandidate[]>([]);
  let progress = $state<ImportProgress | null>(null);
  let errorMsg = $state("");
  let collapsed = $state(new Set<string>());

  // ── Import options ────────────────────────────────────────────────────────

  const FORMAT_GROUPS: { key: string; label: string; exts: string[] }[] = [
    { key: "fits", label: "FITS", exts: ["fit", "fits"] },
    { key: "png", label: "PNG", exts: ["png"] },
    { key: "jpeg", label: "JPEG", exts: ["jpg", "jpeg"] },
    { key: "tiff", label: "TIFF", exts: ["tif", "tiff"] },
  ];

  let selectedFormats = $state(new Set<string>(["fits"]));
  let deleteAfterCopy = $state(false);

  let extensions = $derived(
    FORMAT_GROUPS.filter((g) => selectedFormats.has(g.key)).flatMap((g) => g.exts),
  );

  function toggleFormat(key: string) {
    const next = new Set(selectedFormats);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    selectedFormats = next;
    if (phase === "scanned") scan();
  }

  // ── Tree building ─────────────────────────────────────────────────────────

  interface FolderNode {
    name: string;
    folderPath: string;
    subfolders: FolderNode[];
    files: ImportCandidate[];
  }

  function buildTree(items: ImportCandidate[]): FolderNode {
    const root: FolderNode = { name: "", folderPath: "", subfolders: [], files: [] };
    for (const c of items) {
      const parts = c.relativePath.split(/[\\/]/);
      let node = root;
      for (let i = 0; i < parts.length - 1; i++) {
        const seg = parts[i];
        let child = node.subfolders.find((f) => f.name === seg);
        if (!child) {
          const fp = node.folderPath ? node.folderPath + "/" + seg : seg;
          child = { name: seg, folderPath: fp, subfolders: [], files: [] };
          node.subfolders.push(child);
        }
        node = child;
      }
      node.files.push(c);
    }
    return root;
  }

  function nodeFileCount(node: FolderNode): number {
    return node.files.length + node.subfolders.reduce((s, f) => s + nodeFileCount(f), 0);
  }

  function nodeSize(node: FolderNode): number {
    return (
      node.files.reduce((s, c) => s + c.fileSize, 0) +
      node.subfolders.reduce((s, f) => s + nodeSize(f), 0)
    );
  }

  type FlatRow =
    | {
        kind: "folder";
        depth: number;
        name: string;
        folderPath: string;
        count: number;
        size: number;
      }
    | { kind: "file"; depth: number; candidate: ImportCandidate };

  function flattenTree(node: FolderNode, depth: number, col: Set<string>): FlatRow[] {
    const rows: FlatRow[] = [];
    for (const sub of node.subfolders) {
      rows.push({
        kind: "folder",
        depth,
        name: sub.name,
        folderPath: sub.folderPath,
        count: nodeFileCount(sub),
        size: nodeSize(sub),
      });
      if (!col.has(sub.folderPath)) {
        rows.push(...flattenTree(sub, depth + 1, col));
      }
    }
    for (const c of node.files) {
      rows.push({ kind: "file", depth, candidate: c });
    }
    return rows;
  }

  let tree = $derived(buildTree(candidates));
  let flatRows = $derived(flattenTree(tree, 0, collapsed));

  function toggleFolder(fp: string) {
    const next = new Set(collapsed);
    if (next.has(fp)) next.delete(fp);
    else next.add(fp);
    collapsed = next;
  }

  function collapseAll() {
    const fps = new Set<string>();
    function collect(node: FolderNode) {
      for (const sub of node.subfolders) {
        fps.add(sub.folderPath);
        collect(sub);
      }
    }
    collect(tree);
    collapsed = fps;
  }

  function expandAll() {
    collapsed = new Set();
  }

  // ── Actions ───────────────────────────────────────────────────────────────

  async function selectSource() {
    const path = await SelectSourceFolder();
    if (!path) return;
    sourceFolder = path;
    collapsed = new Set();
    await scan();
  }

  async function scan() {
    phase = "scanning";
    errorMsg = "";
    try {
      const result = await ScanImportCandidates(sourceFolder, extensions);
      candidates = result ?? [];
      phase = "scanned";
    } catch (e) {
      errorMsg = String(e);
      phase = "error";
    }
  }

  async function startImport() {
    phase = "importing";
    progress = null;
    try {
      await StartImport(sourceFolder, extensions, deleteAfterCopy);
    } catch (e) {
      errorMsg = String(e);
      phase = "error";
    }
  }

  function reset() {
    phase = "idle";
    sourceFolder = "";
    candidates = [];
    progress = null;
    errorMsg = "";
    collapsed = new Set();
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }

  let totalSize = $derived(candidates.reduce((sum, c) => sum + c.fileSize, 0));
  let progressPct = $derived(
    progress && progress.total > 0 ? Math.round((progress.current / progress.total) * 100) : 0,
  );

  onMount(() => {
    EventsOn("import:progress", (data: ImportProgress) => {
      progress = data;
      if (data.phase === "done") phase = "done";
      else if (data.phase === "error") {
        errorMsg = data.error ?? "Unknown error";
        phase = "error";
      }
    });
  });

  onDestroy(() => {
    EventsOff("import:progress");
  });
</script>

<div class="import-view">
  {#if phase === "idle" || phase === "error"}
    <div class="center-panel">
      <div class="import-icon">⇪</div>
      <p class="panel-title">Import Files</p>
      <p class="panel-sub">
        Select a source folder to find files not yet in your library and copy them to
        <strong>{rootFolder}</strong>
      </p>
      <div class="format-row">
        <span class="format-label">Import:</span>
        {#each FORMAT_GROUPS as g}
          <button
            class="fmt-btn"
            class:active={selectedFormats.has(g.key)}
            onclick={() => toggleFormat(g.key)}>{g.label}</button
          >
        {/each}
      </div>
      <button
        class="btn-primary btn-large"
        disabled={selectedFormats.size === 0}
        onclick={selectSource}>Select Source Folder</button
      >
      {#if errorMsg}
        <p class="error-msg">{errorMsg}</p>
      {/if}
    </div>
  {:else if phase === "scanning"}
    <div class="center-panel">
      <div class="import-icon spinning">⇪</div>
      <p class="panel-title">Scanning…</p>
      <p class="panel-sub">{sourceFolder}</p>
    </div>
  {:else if phase === "scanned"}
    <div class="scanned-panel">
      <div class="scan-header">
        <div class="scan-header-row">
          <div class="source-info">
            <span class="source-label">Source:</span>
            <span class="source-path" title={sourceFolder}>{sourceFolder}</span>
          </div>
          <div class="scan-actions">
            {#if candidates.length > 0}
              <button class="btn-ghost" onclick={expandAll} title="Expand all folders">⊞</button>
              <button class="btn-ghost" onclick={collapseAll} title="Collapse all folders">⊟</button
              >
            {/if}
            <button class="btn-secondary" onclick={selectSource}>Change Folder</button>
            {#if candidates.length > 0}
              <button class="btn-primary" class:btn-danger={deleteAfterCopy} onclick={startImport}>
                Import {candidates.length}
                {candidates.length === 1 ? "file" : "files"}
              </button>
            {/if}
          </div>
        </div>
        <div class="scan-options">
          <span class="format-label">Formats:</span>
          {#each FORMAT_GROUPS as g}
            <button
              class="fmt-btn"
              class:active={selectedFormats.has(g.key)}
              onclick={() => toggleFormat(g.key)}>{g.label}</button
            >
          {/each}
          <div class="opt-sep"></div>
          <label class="delete-toggle" class:delete-active={deleteAfterCopy}>
            <input type="checkbox" bind:checked={deleteAfterCopy} />
            Delete from source after copying
          </label>
        </div>
      </div>

      {#if candidates.length === 0}
        <div class="empty-state">
          <p>All files are already in the library. Nothing to import.</p>
        </div>
      {:else}
        <div class="candidate-meta">
          {candidates.length}
          {candidates.length === 1 ? "new file" : "new files"} · {formatSize(totalSize)} total
        </div>
        <div class="candidate-list">
          <table>
            <thead>
              <tr>
                <th>Source</th>
                <th>Destination</th>
                <th class="col-size">Size</th>
              </tr>
            </thead>
            <tbody>
              {#each flatRows as row}
                {#if row.kind === "folder"}
                  <tr
                    class="folder-row"
                    onclick={() => toggleFolder(row.folderPath)}
                    title={row.folderPath}
                  >
                    <td style="padding-left: {row.depth * 18 + 10}px">
                      <span class="chevron">{collapsed.has(row.folderPath) ? "▶" : "▼"}</span>
                      <span class="folder-name">{row.name}</span>
                      <span class="folder-meta"
                        >{row.count} {row.count === 1 ? "file" : "files"}</span
                      >
                    </td>
                    <td class="dest-path folder-dest">→ {row.name}/</td>
                    <td class="size-cell">{formatSize(row.size)}</td>
                  </tr>
                {:else}
                  <tr class="file-row">
                    <td style="padding-left: {row.depth * 18 + 28}px" class="file-name">
                      {row.candidate.relativePath.split(/[\\/]/).pop()}
                    </td>
                    <td class="dest-path">{row.candidate.destPath}</td>
                    <td class="size-cell">{formatSize(row.candidate.fileSize)}</td>
                  </tr>
                {/if}
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>
  {:else if phase === "importing"}
    <div class="center-panel">
      <div class="import-icon">⇪</div>
      <p class="panel-title">Importing…</p>
      {#if progress}
        <div class="progress-wrap">
          <div class="progress-bar" style="width: {progressPct}%"></div>
        </div>
        <p class="progress-counts">{progress.current} / {progress.total}</p>
        <p class="progress-file">{progress.currentFile}</p>
      {/if}
    </div>
  {:else if phase === "done"}
    <div class="center-panel">
      <div class="import-icon done-icon">✓</div>
      <p class="panel-title">Import Complete</p>
      {#if progress}
        <p class="panel-sub">{progress.copied} copied · {progress.skipped} already existed</p>
      {/if}
      <button class="btn-primary btn-large" onclick={reset}>Import More</button>
    </div>
  {/if}
</div>

<style>
  .import-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-base);
  }

  /* ── Centred panels ──────────────────────────────────────────────────────── */

  .center-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 32px;
    text-align: center;
  }

  .import-icon {
    font-size: 2.8rem;
    color: var(--accent-dim);
    margin-bottom: 6px;
    line-height: 1;
  }

  .import-icon.spinning {
    animation: spin 1.2s linear infinite;
  }

  .import-icon.done-icon {
    color: var(--accent);
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .panel-title {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .panel-sub {
    font-size: 0.875rem;
    color: var(--text-secondary);
    max-width: 480px;
    margin: 0;
  }

  .error-msg {
    font-size: 0.85rem;
    color: var(--color-error, #e06c75);
    margin: 0;
    max-width: 480px;
  }

  /* ── Progress ────────────────────────────────────────────────────────────── */

  .progress-wrap {
    width: 320px;
    height: 6px;
    background: var(--bg-row);
    border-radius: 3px;
    overflow: hidden;
    margin-top: 8px;
  }

  .progress-bar {
    height: 100%;
    background: var(--accent);
    border-radius: 3px;
    transition: width 0.1s ease;
  }

  .progress-counts {
    font-size: 0.85rem;
    color: var(--text-secondary);
    margin: 4px 0 0;
  }

  .progress-file {
    font-size: 0.78rem;
    color: var(--text-secondary);
    max-width: 420px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    margin: 0;
  }

  /* ── Scanned panel ───────────────────────────────────────────────────────── */

  .scanned-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .scan-header {
    display: flex;
    flex-direction: column;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-panel);
    gap: 6px;
    flex-shrink: 0;
  }

  .scan-header-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .source-info {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  .source-label {
    font-size: 0.8rem;
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .source-path {
    font-size: 0.8rem;
    color: var(--text-primary);
    font-family: monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .scan-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  /* ── Format + options row ────────────────────────────────────────────────── */

  .format-row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .scan-options {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }

  .format-label {
    font-size: 0.75rem;
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .fmt-btn {
    font-size: 0.72rem;
    font-weight: 600;
    padding: 2px 9px;
    border-radius: 3px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition:
      background 0.1s,
      color 0.1s,
      border-color 0.1s;
  }
  .fmt-btn:hover {
    border-color: var(--accent);
    color: var(--text-primary);
  }
  .fmt-btn.active {
    border-color: var(--accent);
    background: var(--accent-dim);
    color: var(--accent);
  }

  .opt-sep {
    width: 1px;
    height: 14px;
    background: var(--border);
    margin: 0 4px;
    flex-shrink: 0;
  }

  .delete-toggle {
    display: flex;
    align-items: center;
    gap: 5px;
    font-size: 0.75rem;
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
  }
  .delete-toggle input {
    accent-color: var(--danger);
    cursor: pointer;
  }
  .delete-toggle.delete-active {
    color: var(--danger);
  }

  .btn-danger {
    background: var(--danger) !important;
    border-color: var(--danger) !important;
  }

  .candidate-meta {
    padding: 6px 16px;
    font-size: 0.8rem;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    background: var(--bg-panel);
  }

  .empty-state {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
    font-size: 0.9rem;
  }

  /* ── Tree table ──────────────────────────────────────────────────────────── */

  .candidate-list {
    flex: 1;
    overflow-y: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }

  thead th {
    position: sticky;
    top: 0;
    background: var(--bg-panel);
    color: var(--text-secondary);
    font-weight: 500;
    text-align: left;
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    user-select: none;
    white-space: nowrap;
  }

  .col-size {
    width: 80px;
    text-align: right;
  }

  /* Folder rows */

  .folder-row {
    cursor: pointer;
    background: var(--bg-panel);
  }

  .folder-row:hover {
    background: var(--bg-row-hover);
  }

  .folder-row td {
    padding: 5px 12px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    font-weight: 500;
    vertical-align: middle;
    white-space: nowrap;
  }

  .chevron {
    display: inline-block;
    font-size: 0.6rem;
    width: 12px;
    color: var(--text-secondary);
    margin-right: 4px;
  }

  .folder-name {
    color: var(--accent);
  }

  .folder-meta {
    margin-left: 8px;
    font-size: 0.75rem;
    color: var(--text-secondary);
    font-weight: 400;
  }

  .folder-dest {
    font-size: 0.78rem;
    font-family: monospace;
    color: var(--accent-dim);
  }

  /* File rows */

  .file-row:hover {
    background: var(--bg-row-hover);
  }

  .file-row td {
    padding: 4px 12px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    vertical-align: middle;
  }

  .file-name {
    font-family: monospace;
    font-size: 0.78rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 320px;
  }

  .dest-path {
    font-family: monospace;
    font-size: 0.78rem;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 360px;
  }

  .size-cell {
    text-align: right;
    color: var(--text-secondary);
    white-space: nowrap;
  }

  /* ── Shared ──────────────────────────────────────────────────────────────── */

  .btn-ghost {
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    font-size: 0.85rem;
    padding: 3px 7px;
    cursor: pointer;
    transition:
      color 0.15s,
      border-color 0.15s;
  }

  .btn-ghost:hover {
    color: var(--text-primary);
    border-color: var(--accent-dim);
  }

  .btn-large {
    padding: 9px 24px;
    font-size: 0.9rem;
  }
</style>
