<script lang="ts">
  import { collapsedFolders, type Candidate } from "../../lib/import/session.svelte";
  import { allFolderPaths, buildTree, flattenTree } from "../../lib/import/tree";
  import { formatBytes } from "../../lib/utils";
  import Spinner from "./Spinner.svelte";

  interface Props {
    candidates: Candidate[];
    rootFolder: string;
    /** A re-scan is in flight: keep showing the current tree with an inline indicator. */
    updating?: boolean;
  }
  let { candidates, rootFolder, updating = false }: Props = $props();

  let tree = $derived(buildTree(candidates));
  let rows = $derived(flattenTree(tree, collapsedFolders, rootFolder));
  let totalSize = $derived(candidates.reduce((s, c) => s + c.fileSize, 0));
  let rootName = $derived(rootFolder.split(/[\\/]/).filter(Boolean).pop() ?? "library");

  function toggleFolder(fp: string) {
    if (collapsedFolders.has(fp)) collapsedFolders.delete(fp);
    else collapsedFolders.add(fp);
  }
  function collapseAll() {
    for (const fp of allFolderPaths(tree)) collapsedFolders.add(fp);
  }
  function expandAll() {
    collapsedFolders.clear();
  }
</script>

<div class="tree-meta">
  <span>
    {candidates.length.toLocaleString()}
    {candidates.length === 1 ? "new file" : "new files"} · {formatBytes(totalSize)} total
  </span>
  {#if updating}
    <span class="updating"><Spinner size={12} label="Updating" /> Updating…</span>
  {/if}
  <span class="spacer"></span>
  <button class="tool-btn" onclick={expandAll} title="Expand all folders">Expand all</button>
  <button class="tool-btn" onclick={collapseAll} title="Collapse all folders">Collapse all</button>
</div>

<div class="tree-scroll" class:dimmed={updating} aria-busy={updating}>
  <table>
    <thead>
      <tr>
        <th>Source</th>
        <th title="Path inside your library folder ({rootFolder})">Destination in {rootName}/</th>
        <th class="col-size">Size</th>
      </tr>
    </thead>
    <tbody>
      {#each rows as row (row.kind === "folder" ? "d:" + row.folderPath : "f:" + row.candidate.sourcePath)}
        {#if row.kind === "folder"}
          <tr class="folder-row">
            <td style:padding-left="{row.depth * 18 + 10}px">
              <button
                class="folder-toggle"
                aria-expanded={!collapsedFolders.has(row.folderPath)}
                title={row.folderPath}
                onclick={() => toggleFolder(row.folderPath)}
              >
                <span class="chevron" aria-hidden="true"
                  >{collapsedFolders.has(row.folderPath) ? "▸" : "▾"}</span
                >
                <span class="folder-name">{row.name}</span>
              </button>
              <span class="folder-meta">{row.count} {row.count === 1 ? "file" : "files"}</span>
            </td>
            <td class="dest-path">
              {#if row.destFolder !== null}
                <span
                  class="folder-dest"
                  title="Files directly in “{row.name}” are copied to {row.destFolder ||
                    rootName}/ in your library. Only the immediate parent folder is kept — deeper source folder structure is flattened."
                  >→ {row.destFolder ? row.destFolder + "/" : "(library root)"}</span
                >
              {/if}
            </td>
            <td class="size-cell">{formatBytes(row.size)}</td>
          </tr>
        {:else}
          <tr class="file-row">
            <td style:padding-left="{row.depth * 18 + 32}px" class="file-name" title={row.name}
              >{row.name}</td
            >
            <td class="dest-path" title={row.candidate.destPath}>{row.dest}</td>
            <td class="size-cell">{formatBytes(row.candidate.fileSize)}</td>
          </tr>
        {/if}
      {/each}
    </tbody>
  </table>
</div>

<style>
  .tree-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 16px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border);
    background: var(--bg-panel);
    flex-shrink: 0;
  }
  .updating {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--accent);
  }
  .spacer {
    flex: 1;
  }

  .tree-scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    transition: filter 0.15s;
  }
  .tree-scroll.dimmed {
    filter: saturate(0.6);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
    font-size: var(--fs-sm);
  }
  thead th {
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--bg-panel);
    color: var(--text-secondary);
    font-weight: 500;
    text-align: left;
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .col-size {
    width: 90px;
    text-align: right;
  }

  .folder-row {
    background: var(--bg-panel);
  }
  .folder-row:hover {
    background: var(--bg-row-hover);
  }
  .folder-row td {
    padding: 4px 12px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .folder-toggle {
    background: none;
    border: none;
    padding: 0;
    color: inherit;
    font: inherit;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .chevron {
    display: inline-block;
    width: 12px;
    color: var(--text-secondary);
  }
  .folder-name {
    color: var(--accent);
  }
  .folder-meta {
    margin-left: 8px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-weight: 400;
  }
  .folder-dest {
    color: var(--text-secondary);
    font-style: italic;
    cursor: help;
  }

  .file-row:hover {
    background: var(--bg-row-hover);
  }
  .file-row td {
    padding: 4px 12px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
  }
  .file-name,
  .dest-path {
    font-family: monospace;
    font-size: var(--fs-xs);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .dest-path {
    color: var(--text-secondary);
  }
  .size-cell {
    text-align: right;
    color: var(--text-secondary);
    white-space: nowrap;
  }
</style>
