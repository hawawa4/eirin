<script lang="ts">
  import {
    session,
    selectedFormats,
    ensureListening,
    syncStatus,
    chooseSource,
    startImport,
    cancelImport,
    resetSession,
    isRunningPhase,
  } from "../lib/import/session.svelte";
  import FormatToggles from "./import/FormatToggles.svelte";
  import CandidateTree from "./import/CandidateTree.svelte";
  import ImportRunning from "./import/ImportRunning.svelte";
  import ImportResult from "./import/ImportResult.svelte";
  import DeleteSourceModal from "./import/DeleteSourceModal.svelte";
  import Spinner from "./import/Spinner.svelte";

  interface Props {
    rootFolder: string;
    /** True while this view's tab is visible. */
    active?: boolean;
    /** Request a library (re)scan/index. */
    onscan?: () => void;
    /** Switch to the Library tab, e.g. from the import-done screen. */
    onviewlibrary?: () => void;
  }

  let { rootFolder, active = true, onscan, onviewlibrary }: Props = $props();

  ensureListening();

  // Pick the job back up whenever the tab is (re)shown: a remount or a missed event
  // must never offer a second import while one is running.
  $effect(() => {
    if (active !== false) void syncStatus();
  });

  let confirmDelete = $state(false);

  let running = $derived(isRunningPhase(session.progress?.phase));
  let count = $derived(session.candidates.length);
  let canImport = $derived(
    count > 0 &&
      !!rootFolder &&
      selectedFormats.size > 0 &&
      !session.starting &&
      !session.rescanning &&
      !running,
  );

  function requestImport() {
    if (!canImport) return;
    if (session.deleteAfterCopy) confirmDelete = true;
    else void startImport();
  }

  function retry() {
    session.screen = "review";
    requestImport();
  }
</script>

<div class="import-view">
  {#if session.screen === "pick"}
    <div class="center-panel">
      <div class="import-icon" aria-hidden="true">⇪</div>
      <p class="panel-title">Import Files</p>
      {#if rootFolder}
        <p class="panel-sub">
          Select a source folder (e.g. your Seestar) to find files that aren't in your library yet
          and copy them to <code>{rootFolder}</code>.
        </p>
      {:else}
        <p class="panel-sub">No library folder is set. Choose one in Settings before importing.</p>
      {/if}
      <FormatToggles label="Import:" />
      <button
        class="btn-primary btn-large"
        disabled={selectedFormats.size === 0 || !rootFolder}
        title={selectedFormats.size === 0 ? "Select at least one file format" : undefined}
        onclick={chooseSource}>Select Source Folder</button
      >
      {#if session.scanError}
        <p class="error-msg" role="alert">Couldn't scan the folder: {session.scanError}</p>
      {/if}
    </div>
  {:else if session.screen === "scanning"}
    <div class="center-panel">
      <Spinner size={36} label="Scanning" />
      <p class="panel-title">Scanning…</p>
      <p class="panel-sub mono">{session.sourceFolder}</p>
    </div>
  {:else if session.screen === "review"}
    <div class="review">
      <div class="review-header">
        <div class="review-row">
          <div class="source-info">
            <span class="source-label">Source</span>
            <span class="source-path" title={session.sourceFolder}>{session.sourceFolder}</span>
          </div>
          <div class="review-actions">
            <button class="btn-secondary" onclick={chooseSource} disabled={session.starting}
              >Change folder</button
            >
            {#if count > 0}
              <button
                class={session.deleteAfterCopy ? "btn-danger" : "btn-primary"}
                disabled={!canImport}
                onclick={requestImport}
              >
                {#if session.starting}
                  Starting…
                {:else}
                  {session.deleteAfterCopy ? "Import & delete" : "Import"}
                  {count.toLocaleString()}
                  {count === 1 ? "file" : "files"}
                {/if}
              </button>
            {/if}
          </div>
        </div>
        <div class="review-row options">
          <FormatToggles />
          <span class="opt-sep" aria-hidden="true"></span>
          <label
            class="delete-toggle"
            class:delete-active={session.deleteAfterCopy}
            title="After copying, delete each file from the source — only once the library copy is verified identical"
          >
            <input type="checkbox" bind:checked={session.deleteAfterCopy} />
            Delete from source after a verified copy
          </label>
        </div>
        {#if session.startError}
          <p class="error-msg" role="alert">Couldn't start the import: {session.startError}</p>
        {/if}
        {#if session.scanError}
          <p class="error-msg" role="alert">Couldn't rescan the folder: {session.scanError}</p>
        {/if}
      </div>

      {#if count === 0}
        <div class="empty-state">
          {#if session.rescanning}
            <Spinner size={16} label="Updating" /> Updating…
          {:else if selectedFormats.size === 0}
            Select at least one file format.
          {:else}
            All files are already in the library. Nothing to import.
          {/if}
        </div>
      {:else}
        <CandidateTree candidates={session.candidates} {rootFolder} updating={session.rescanning} />
      {/if}
    </div>
  {:else if session.screen === "running"}
    <ImportRunning
      progress={session.progress}
      cancelling={session.cancelling}
      oncancel={cancelImport}
    />
  {:else if session.screen === "result" && session.progress}
    <ImportResult
      progress={session.progress}
      canRetry={!!session.sourceFolder}
      retrying={session.starting}
      onretry={retry}
      onagain={resetSession}
      {onviewlibrary}
      {onscan}
    />
  {/if}
</div>

{#if confirmDelete}
  <DeleteSourceModal
    {count}
    sourceFolder={session.sourceFolder}
    oncancel={() => (confirmDelete = false)}
    onconfirm={() => {
      confirmDelete = false;
      void startImport();
    }}
  />
{/if}

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
    color: var(--accent);
    margin-bottom: 6px;
    line-height: 1;
  }
  .panel-title {
    font-size: var(--fs-xl);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }
  .panel-sub {
    font-size: var(--fs-md);
    color: var(--text-secondary);
    max-width: 520px;
    margin: 0 0 6px;
    word-break: break-word;
  }
  .panel-sub code,
  .mono {
    font-family: monospace;
    color: var(--text-primary);
  }
  .error-msg {
    font-size: var(--fs-sm);
    color: var(--danger);
    margin: 0;
    max-width: 640px;
    word-break: break-word;
  }

  /* ── Review ──────────────────────────────────────────────────────────────── */

  .review {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-height: 0;
  }
  .review-header {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-panel);
    flex-shrink: 0;
  }
  .review-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }
  .review-row.options {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
  .source-info {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .source-label {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    flex-shrink: 0;
  }
  .source-path {
    font-size: var(--fs-sm);
    color: var(--text-primary);
    font-family: monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .review-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }
  .review-actions .btn-secondary {
    margin-right: 0;
  }
  .opt-sep {
    width: 1px;
    height: 16px;
    background: var(--border);
    margin: 0 6px;
  }
  .delete-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: var(--fs-md);
    color: var(--text-primary);
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

  .empty-state {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--text-secondary);
    font-size: var(--fs-md);
  }
</style>
