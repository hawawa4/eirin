<script lang="ts">
  import type { AppMode } from "../lib/types";
  import { MODES, modeUnavailableReason, type ModeDef, type ModeGroup } from "../lib/shell/modes";

  interface Props {
    mode: AppMode;
    rootFolder: string;
    onmodechange: (mode: AppMode) => void;
  }

  let { mode, rootFolder, onmodechange }: Props = $props();

  const GROUPS: { id: Exclude<ModeGroup, "settings">; label: string }[] = [
    { id: "visualize", label: "Visualize" },
    { id: "manage", label: "Manage" },
  ];

  let tabs = $derived(MODES.filter((m) => m.group !== "settings"));

  /** Tooltip: what the tab is for, plus its Ctrl+digit shortcut. */
  function tooltip(m: ModeDef, reason: string | null): string {
    if (reason) return reason;
    const n = tabs.indexOf(m) + 1;
    return n > 0 && n <= 9 ? `${m.label} — ${m.hint} (Ctrl+${n})` : `${m.label} — ${m.hint}`;
  }
</script>

<nav class="mode-selector" aria-label="Views">
  {#each GROUPS as g (g.id)}
    {@const items = tabs.filter((m) => m.group === g.id)}
    {#if items.length > 0}
      <div
        class="mode-group"
        class:visualize={g.id === "visualize"}
        role="group"
        aria-label={g.label}
      >
        {#each items as m (m.value)}
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
            title={tooltip(m, reason)}
          >
            <span class="mode-icon" aria-hidden="true">{m.icon}</span>
            <span class="mode-label">{m.label}</span>
          </button>
        {/each}
      </div>
    {/if}
  {/each}
</nav>

<style>
  .mode-selector {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }

  .mode-group {
    display: flex;
    gap: 2px;
    background: var(--bg-base);
    border-radius: 6px;
    padding: 2px;
    border: 1px solid var(--border);
    min-width: 0;
  }

  /* The looking-at-it tabs read as a lighter, separate cluster. */
  .mode-group.visualize {
    background: color-mix(in srgb, var(--bg-base) 70%, var(--accent) 8%);
    border-color: color-mix(in srgb, var(--border) 60%, var(--accent) 40%);
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

  /* Narrow windows: icons only (the tooltip still names the tab). */
  @media (max-width: 1100px) {
    .mode-btn:not(.active) .mode-label {
      display: none;
    }
    .mode-btn {
      padding: 5px 9px;
    }
  }
</style>
