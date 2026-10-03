<script lang="ts">
  import { ui, type UiScale } from "../../lib/uiState.svelte";

  const OPTIONS: { value: UiScale; label: string }[] = [
    { value: 14, label: "Small" },
    { value: 16, label: "Default" },
    { value: 18, label: "Large" },
  ];
</script>

<section class="card">
  <h2 class="section-title">Appearance</h2>

  <div class="scale-row">
    <span class="scale-label" id="ui-scale-label">UI scale</span>
    <div class="segmented" role="group" aria-labelledby="ui-scale-label">
      {#each OPTIONS as o (o.value)}
        <button
          class="tool-btn seg-btn"
          class:active={ui.uiScale === o.value}
          aria-pressed={ui.uiScale === o.value}
          onclick={() => (ui.uiScale = o.value)}
        >
          {o.label}
        </button>
      {/each}
    </div>
  </div>
  <p class="action-hint">
    Scales all text and controls. Shortcuts anywhere in the app: <kbd>Ctrl</kbd> + <kbd>+</kbd>
    larger,
    <kbd>Ctrl</kbd> + <kbd>−</kbd> smaller, <kbd>Ctrl</kbd> + <kbd>0</kbd> reset.
  </p>
</section>

<style>
  .scale-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .scale-label {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    min-width: 64px;
  }

  .segmented {
    display: flex;
    gap: 4px;
  }

  .seg-btn {
    font-size: var(--fs-sm);
    padding: 4px 12px;
  }

  kbd {
    font-family: monospace;
    font-size: var(--fs-xs);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 0 4px;
    color: var(--text-primary);
  }
</style>
