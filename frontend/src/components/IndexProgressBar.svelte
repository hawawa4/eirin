<script lang="ts">
  import type { IndexProgress } from '../lib/types'

  interface Props {
    progress: IndexProgress
    oncancel:  () => void
  }

  let { progress, oncancel }: Props = $props()
</script>

<div class="index-bar">
  <span class="index-phase">
    {#if progress.phase === 'scanning'}
      Scanning directories…
    {:else}
      Indexing {progress.done} / {progress.total}
      {#if progress.indexed > 0}
        <span class="index-new">+{progress.indexed} new</span>
      {/if}
    {/if}
  </span>
  <div class="index-track">
    <div
      class="index-fill"
      style="width: {progress.total > 0 ? (progress.done / progress.total * 100).toFixed(1) : 0}%"
    ></div>
  </div>
  <span class="index-file" title={progress.current}>{progress.current}</span>
  <button class="tool-btn" onclick={oncancel}>Cancel</button>
</div>

<style>
  .index-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 14px;
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
    border-bottom: 1px solid var(--border-accent);
    flex-shrink: 0;
    font-size: 0.78rem;
  }

  .index-phase {
    color: var(--text-primary);
    white-space: nowrap;
    min-width: 120px;
  }

  .index-new {
    color: var(--success);
    margin-left: 4px;
    font-size: 0.74rem;
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
    color: var(--text-dim);
    font-family: 'Consolas', 'Fira Code', monospace;
    font-size: 0.72rem;
    max-width: 240px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
