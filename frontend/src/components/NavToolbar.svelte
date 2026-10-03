<script lang="ts">
  import { truncatePath } from "../lib/utils";

  interface Props {
    currentPath: string;
    canGoBack: boolean;
    onnavigateBack: () => void;
    /** "Show in folder" for the current directory — hidden when absent (e.g. server mode). */
    onreveal?: () => void;
  }

  let { currentPath, canGoBack, onnavigateBack, onreveal }: Props = $props();
</script>

<div class="toolbar">
  <button
    class="btn-icon"
    onclick={onnavigateBack}
    disabled={!canGoBack}
    title="Go back"
    aria-label="Go back">←</button
  >
  <span class="path-display" title={currentPath}>{truncatePath(currentPath)}</span>
  {#if onreveal && currentPath}
    <button
      class="tool-btn reveal-btn"
      onclick={onreveal}
      title="Open this folder in the file manager">Show in folder</button
    >
  {/if}
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 16px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .path-display {
    flex: 1;
    min-width: 0;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    font-family: "Consolas", "Fira Code", monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .reveal-btn {
    flex-shrink: 0;
  }
</style>
