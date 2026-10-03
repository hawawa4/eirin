<script lang="ts">
  import type * as app from "$models/app";
  import { RevealPath } from "$app";
  import { attempt } from "../../lib/toast.svelte";
  import { formatBytes } from "../../lib/utils";

  interface Props {
    folder: string;
    files: app.ProjectOutputFile[];
    /** First poll finished (success or failure). */
    loaded: boolean;
    error: string;
    onsave: () => void;
  }
  let { folder, files, loaded, error, onsave }: Props = $props();
</script>

<section class="outputs">
  <div class="outputs-hdr">
    <span class="title">Outputs</span>
    {#if loaded && !error}<span class="count">{files.length}</span>{/if}
    <span class="pulse-dot" title="Watching the project folder for new files"></span>
    <span class="spacer"></span>
    {#if files.length > 0}
      <button
        class="btn-secondary small"
        onclick={onsave}
        title="Copy processed results from the project folder into your library"
        >Save outputs to library…</button
      >
    {/if}
  </div>

  <div class="body">
    {#if error && files.length === 0}
      <p class="hint error" role="alert">Couldn't read the project folder: {error}</p>
    {:else if !loaded}
      <p class="hint">Checking for output files…</p>
    {:else if files.length === 0}
      <p class="hint">
        No output files yet. Process the frames in Siril and save the results in
        <code>{folder}</code> — they appear here automatically.
      </p>
    {:else}
      {#if error}<p class="hint error">Couldn't refresh: {error}</p>{/if}
      <table class="file-table">
        <thead>
          <tr>
            <th>Filename</th>
            <th class="col-size">Size</th>
            <th class="col-date">Modified</th>
          </tr>
        </thead>
        <tbody>
          {#each files as f (f.path)}
            <tr>
              <td class="mono-cell">
                <button
                  class="file-link"
                  title="Show in file manager"
                  onclick={() => attempt(() => RevealPath(f.path), "Couldn't show the file")}
                  >{f.name}</button
                >
              </td>
              <td class="col-size dim-cell">{formatBytes(f.size)}</td>
              <td class="col-date dim-cell">{f.modTime.slice(0, 10)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</section>

<style>
  .outputs-hdr {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 24px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
  }
  .title {
    font-size: var(--fs-sm);
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .count {
    font-size: var(--fs-sm);
    color: var(--accent);
  }
  .spacer {
    flex: 1;
  }
  .small {
    font-size: var(--fs-xs);
    padding: 3px 10px;
    margin-right: 0;
  }
  .pulse-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    animation: pulse 2.5s ease-in-out infinite;
    flex-shrink: 0;
  }
  @keyframes pulse {
    0%,
    100% {
      opacity: 0.3;
    }
    50% {
      opacity: 0.9;
    }
  }

  .body {
    padding: 10px 24px 16px;
  }
  .hint {
    font-size: var(--fs-md);
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.5;
  }
  .hint.error {
    color: var(--danger);
    margin-bottom: 8px;
  }
  .hint code {
    font-family: monospace;
    color: var(--text-primary);
    font-size: var(--fs-sm);
    word-break: break-all;
  }

  .file-table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--fs-md);
    border: 1px solid var(--border);
  }
  .file-table th {
    padding: 5px 10px;
    text-align: left;
    font-size: var(--fs-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
  }
  .file-table td {
    padding: 5px 10px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
  }
  .mono-cell {
    font-family: monospace;
    word-break: break-all;
  }
  .file-link {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: var(--text-primary);
    cursor: pointer;
    text-align: left;
  }
  .file-link:hover {
    color: var(--accent);
    text-decoration: underline;
  }
  .dim-cell {
    color: var(--text-secondary);
    font-size: var(--fs-sm);
  }
  .col-size {
    text-align: right;
    width: 90px;
  }
  .col-date {
    width: 110px;
    font-variant-numeric: tabular-nums;
  }
</style>
