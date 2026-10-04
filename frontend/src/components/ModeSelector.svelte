<script lang="ts">
  import type { AppMode } from "../lib/types";
  import { visibleModes, modeUnavailableReason } from "../lib/shell/modes";

  interface Props {
    mode: AppMode;
    rootFolder: string;
    readOnly: boolean;
    onmodechange: (mode: AppMode) => void;
  }

  let { mode, rootFolder, readOnly, onmodechange }: Props = $props();
</script>

<nav class="mode-selector" aria-label="Views">
  {#each visibleModes(readOnly) as m (m.value)}
    {@const reason = modeUnavailableReason(m, rootFolder)}
    <!-- aria-disabled (not `disabled`) so the explanatory tooltip still shows on hover. -->
    <button
      class="mode-btn"
      class:active={mode === m.value}
      class:unavailable={reason !== null}
      aria-disabled={reason !== null}
      aria-current={mode === m.value ? "page" : undefined}
      onclick={() => {
        if (reason === null) onmodechange(m.value);
      }}
      title={reason ?? m.label}
    >
      <span class="mode-icon" aria-hidden="true">{m.icon}</span>
      <span class="mode-label">{m.label}</span>
    </button>
  {/each}
</nav>

<style>
  .mode-selector {
    display: flex;
    gap: 2px;
    background: var(--bg-base);
    border-radius: 6px;
    padding: 2px;
    border: 1px solid var(--border);
  }

  .mode-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 12px;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    font-weight: 500;
    cursor: pointer;
    transition:
      background 0.15s,
      color 0.15s;
    white-space: nowrap;
  }

  .mode-btn.unavailable {
    color: var(--text-dim);
    cursor: not-allowed;
  }

  .mode-btn:not(.unavailable):not(.active):hover {
    color: var(--text-primary);
    background: var(--bg-row-hover);
  }

  .mode-btn.active {
    background: var(--accent);
    color: var(--accent-contrast);
    font-weight: 600;
  }

  .mode-icon {
    font-size: var(--fs-md);
    line-height: 1;
  }

  .mode-label {
    line-height: 1;
  }
</style>
