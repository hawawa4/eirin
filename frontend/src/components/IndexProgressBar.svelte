<script lang="ts">
  import type { IndexProgress } from "../lib/types";

  interface Props {
    progress: IndexProgress;
    oncancel: () => void;
  }

  let { progress, oncancel }: Props = $props();

  const CURRENT_MAX = 64;

  let pct = $derived(
    progress.total > 0 ? Math.min(100, Math.round((progress.done / progress.total) * 100)) : 0,
  );

  // Keep the end of the path (the file name) visible: truncate from the start.
  let currentShort = $derived(
    progress.current.length > CURRENT_MAX
      ? "…" + progress.current.slice(-(CURRENT_MAX - 1))
      : progress.current,
  );
</script>

<div class="index-bar">
  <span class="index-phase">
    {#if progress.phase === "scanning"}
      Scanning folders…
    {:else}
      Reading files {progress.done} / {progress.total}
      <span class="index-pct">{pct}%</span>
    {/if}
  </span>
  <div
    class="index-track"
    role="progressbar"
    aria-label="Library scan progress"
    aria-valuemin={0}
    aria-valuemax={100}
    aria-valuenow={progress.phase === "scanning" ? undefined : pct}
  >
    <div class="index-fill" style="width: {pct}%"></div>
  </div>
  {#if progress.indexed > 0}
    <span class="index-new">+{progress.indexed} new</span>
  {/if}
  {#if progress.errors > 0}
    <span class="index-errors" title="Files that couldn't be read"
      >{progress.errors} error{progress.errors !== 1 ? "s" : ""}</span
    >
  {/if}
  <span class="index-file" title={progress.current}>{currentShort}</span>
  <button class="tool-btn" onclick={oncancel}>Cancel</button>
</div>

<style>
  .index-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 14px;
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
    border-bottom: 1px solid var(--border-accent);
    flex-shrink: 0;
    font-size: var(--fs-xs);
  }

  .index-phase {
    color: var(--text-primary);
    white-space: nowrap;
    min-width: 150px;
    font-variant-numeric: tabular-nums;
  }

  .index-pct {
    color: var(--accent);
    margin-left: 4px;
    font-weight: 600;
  }

  .index-new {
    color: var(--success);
    white-space: nowrap;
  }

  .index-errors {
    color: var(--danger);
    white-space: nowrap;
  }

  .index-track {
    flex: 1;
    height: 4px;
    background: var(--border);
    border-radius: 2px;
    overflow: hidden;
    min-width: 60px;
  }

  .index-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 2px;
    transition: width 0.15s linear;
  }

  .index-file {
    color: var(--text-secondary);
    font-family: "Consolas", "Fira Code", monospace;
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
