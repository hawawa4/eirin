<script lang="ts">
  import type { AppMode } from "../lib/types";

  interface ModeOption {
    value: AppMode;
    label: string;
    icon: string;
    requiresRoot: boolean;
  }

  interface Props {
    mode: AppMode;
    rootFolder: string;
    onmodechange: (mode: AppMode) => void;
  }

  let { mode, rootFolder, onmodechange }: Props = $props();

  const modes: ModeOption[] = [
    { value: "browser", label: "Browse", icon: "⊞", requiresRoot: true },
    { value: "library", label: "Library", icon: "◈", requiresRoot: true },
    { value: "import", label: "Import", icon: "⇪", requiresRoot: true },
    { value: "settings", label: "Settings", icon: "⚙", requiresRoot: false },
  ];
</script>

<div class="mode-selector">
  {#each modes as m}
    {@const disabled = m.requiresRoot && !rootFolder}
    <button
      class="mode-btn"
      class:active={mode === m.value}
      onclick={() => !disabled && onmodechange(m.value)}
      {disabled}
      title={disabled ? "Select a root folder first" : m.label}
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
      color 0.15s,
      opacity 0.15s;
    white-space: nowrap;
  }

  .mode-btn:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }

  .mode-btn:not(:disabled):hover {
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
