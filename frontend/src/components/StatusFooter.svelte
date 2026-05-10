<script lang="ts">
  import type { IndexProgress } from '../lib/types'
  import { truncatePath } from '../lib/utils'

  interface Props {
    totalCount:    number
    filteredCount: number
    uncachedCount: number
    indexRunning:  boolean
    indexProgress: IndexProgress | null
    rootFolder:    string
  }

  let { totalCount, filteredCount, uncachedCount, indexRunning, indexProgress, rootFolder }: Props = $props()
</script>

<footer>
  <span>
    {#if filteredCount !== totalCount}
      {filteredCount} of {totalCount} items
    {:else}
      {totalCount} item{totalCount !== 1 ? 's' : ''}
    {/if}
    {#if uncachedCount > 0 && !indexRunning}
      <span class="unindexed-hint" title="Click 'Build Index' to populate FITS metadata">
        · {uncachedCount} not indexed
      </span>
    {/if}
    {#if indexProgress?.phase === 'done'}
      <span class="index-done-hint">· Index up to date</span>
    {/if}
  </span>
  <span class="root-tag">Root: {truncatePath(rootFolder, 50)}</span>
</footer>

<style>
  footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 16px;
    background: var(--bg-panel);
    border-top: 1px solid var(--border);
    font-size: 0.75rem;
    color: var(--text-dim);
    flex-shrink: 0;
  }

  .root-tag { font-family: 'Consolas', 'Fira Code', monospace; color: var(--text-dim); }

  .unindexed-hint {
    color: var(--text-dim);
    opacity: 0.7;
    cursor: default;
  }

  .index-done-hint {
    color: var(--success);
    opacity: 0.8;
  }
</style>
