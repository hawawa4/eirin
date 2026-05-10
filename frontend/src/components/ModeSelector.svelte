<script lang="ts">
  import type { AppMode } from "../lib/types";

  interface ModeOption {
    value: AppMode;
    label: string;
    icon: string;
  }

  interface Props {
    mode: AppMode;
    onmodechange: (mode: AppMode) => void;
  }

  let { mode, onmodechange }: Props = $props();

  const modes: ModeOption[] = [
    { value: "browser", label: "Browse", icon: "⊞" },
    { value: "library", label: "Library", icon: "◈" },
  ];
</script>

<div class="mode-selector">
  {#each modes as m}
    <button
      class="mode-btn"
      class:active={mode === m.value}
      onclick={() => onmodechange(m.value)}
      title={m.label}
    >
      <span class="mode-icon">{m.icon}</span>
      <span class="mode-label">{m.label}</span>
    </button>
  {/each}
</div>

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
    gap: 5px;
    padding: 4px 10px;
    border-radius: 4px;
    border: none;
    background: transparent;
    color: var(--text-secondary);
    font-size: 0.8rem;
    font-weight: 500;
    cursor: pointer;
    transition:
      background 0.15s,
      color 0.15s;
    white-space: nowrap;
  }

  .mode-btn:hover {
    color: var(--text-primary);
    background: var(--bg-row-hover);
  }

  .mode-btn.active {
    background: var(--bg-panel);
    color: var(--accent);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }

  .mode-icon {
    font-size: 0.9rem;
    line-height: 1;
  }

  .mode-label {
    line-height: 1;
  }
</style>
