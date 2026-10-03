<script lang="ts">
  import type { IndexProgress } from "../lib/types";
  import { truncatePath } from "../lib/utils";

  interface Props {
    totalCount: number;
    filteredCount: number;
    uncachedCount: number;
    indexRunning: boolean;
    indexProgress: IndexProgress | null;
    rootFolder: string;
    /** Starts a library scan; the "not indexed" hint becomes a button when set. */
    onscan?: () => void;
  }

  let {
    totalCount,
    filteredCount,
    uncachedCount,
    indexRunning,
    indexProgress,
    rootFolder,
    onscan,
  }: Props = $props();
</script>

<footer>
  <span class="counts">
    {#if filteredCount !== totalCount}
      {filteredCount} of {totalCount} items
    {:else}
      {totalCount} item{totalCount !== 1 ? "s" : ""}
    {/if}
    {#if uncachedCount > 0 && !indexRunning}
      <span class="unindexed-hint">· {uncachedCount} not indexed</span>
      {#if onscan}
        <button
          class="tool-btn"
          onclick={onscan}
          title="Read FITS headers for files not yet in the library">Scan for new files</button
        >
      {/if}
    {:else if indexProgress?.phase === "done" && !indexRunning}
      <span class="index-done-hint">· Index up to date</span>
    {/if}
  </span>
  <span class="root-tag" title={rootFolder}>Root: {truncatePath(rootFolder, 50)}</span>
</footer>

<style>
  footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    padding: 6px 16px;
    background: var(--bg-panel);
    border-top: 1px solid var(--border);
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    flex-shrink: 0;
  }

  .counts {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .root-tag {
    font-family: "Consolas", "Fira Code", monospace;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .unindexed-hint {
    color: var(--text-secondary);
  }

  .index-done-hint {
    color: var(--success);
  }
</style>
