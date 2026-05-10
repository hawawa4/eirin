<script lang="ts">
  import { truncatePath } from '../lib/utils'

  interface Props {
    currentPath:    string
    canGoBack:      boolean
    onnavigateBack: () => void
  }

  let { currentPath, canGoBack, onnavigateBack }: Props = $props()
</script>

<div class="toolbar">
  <button class="btn-icon" onclick={onnavigateBack} disabled={!canGoBack} title="Go back">←</button>
  <span class="path-display" title={currentPath}>{truncatePath(currentPath)}</span>
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
    font-size: 0.82rem;
    color: var(--text-secondary);
    font-family: 'Consolas', 'Fira Code', monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
