<script lang="ts">
  import type { Progress } from "../../lib/import/session.svelte";
  import Spinner from "./Spinner.svelte";

  interface Props {
    progress: Progress | null;
    cancelling: boolean;
    oncancel: () => void;
  }
  let { progress, cancelling, oncancel }: Props = $props();

  let scanningPhase = $derived(!progress || progress.phase === "scanning" || progress.total === 0);
  let pct = $derived(
    progress && progress.total > 0 ? Math.round((progress.current / progress.total) * 100) : 0,
  );
</script>

<div class="running">
  <Spinner size={36} label="Importing" />
  <p class="title">{scanningPhase ? "Preparing import…" : "Importing…"}</p>

  {#if !scanningPhase && progress}
    <div
      class="bar"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={progress.total}
      aria-valuenow={progress.current}
    >
      <div class="fill" style:width="{pct}%"></div>
    </div>
    <p class="counts">
      {progress.current.toLocaleString()} / {progress.total.toLocaleString()} files · {progress.copied}
      copied · {progress.skipped} already in library
    </p>
    {#if progress.currentFile}
      <p class="file" title={progress.currentFile}>{progress.currentFile}</p>
    {/if}
  {:else}
    <p class="counts">Scanning the source folder…</p>
  {/if}

  <button class="btn-ghost danger" disabled={cancelling} onclick={oncancel}>
    {cancelling ? "Cancelling…" : "Cancel import"}
  </button>
  <p class="hint">You can switch tabs — the import keeps running.</p>
</div>

<style>
  .running {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 32px;
    text-align: center;
  }
  .title {
    margin: 6px 0 0;
    font-size: var(--fs-lg);
    font-weight: 600;
    color: var(--text-primary);
  }
  .bar {
    width: min(360px, 80vw);
    height: 6px;
    background: var(--bg-row);
    border-radius: 3px;
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background: var(--accent);
    transition: width 0.15s ease;
  }
  .counts {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
  .file {
    margin: 0;
    font-family: monospace;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    max-width: min(480px, 90vw);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .hint {
    margin: 0;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
</style>
