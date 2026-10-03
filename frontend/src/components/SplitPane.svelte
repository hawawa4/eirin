<script lang="ts">
  import type { Snippet } from "svelte";
  import { ui } from "../lib/uiState.svelte";

  interface Props {
    /** When false only the list is shown, full width. */
    showSecondary: boolean;
    list: Snippet;
    secondary?: Snippet;
    /** Name of the list pane used in the collapse/expand labels (default "list"). */
    collapsedLabel?: string;
    /** List collapsed (hidden) while the secondary pane is shown. Bindable. */
    collapsed?: boolean;
  }

  let {
    showSecondary,
    list,
    secondary,
    collapsedLabel = "list",
    collapsed = $bindable(false),
  }: Props = $props();

  const MIN_PCT = 15;
  const MAX_PCT = 75;
  const KEY_STEP = 2;

  let container: HTMLDivElement;
  let dragging = $state(false);

  let split = $derived(showSecondary && !!secondary);

  let listStyle = $derived(
    split
      ? collapsed
        ? "flex: 0 0 0px; min-width: 0; overflow: hidden;"
        : `flex: 0 0 ${ui.splitPct}%;`
      : "flex: 1;",
  );

  function clamp(pct: number) {
    return Math.max(MIN_PCT, Math.min(MAX_PCT, pct));
  }

  function onDividerMousedown(e: MouseEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    dragging = true;

    function onMove(ev: MouseEvent) {
      const rect = container.getBoundingClientRect();
      ui.splitPct = clamp(((ev.clientX - rect.left) / rect.width) * 100);
      if (collapsed) collapsed = false;
    }
    function onUp() {
      dragging = false;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    }
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  function onDividerKeydown(e: KeyboardEvent) {
    let next: number | null = null;
    if (e.key === "ArrowLeft") next = ui.splitPct - KEY_STEP;
    else if (e.key === "ArrowRight") next = ui.splitPct + KEY_STEP;
    else if (e.key === "Home") next = MIN_PCT;
    else if (e.key === "End") next = MAX_PCT;
    if (next === null) return;
    e.preventDefault();
    ui.splitPct = clamp(next);
    collapsed = false;
  }
</script>

<div class="split" class:dragging bind:this={container}>
  <div class="split-list" style={listStyle} inert={split && collapsed}>
    {@render list()}
  </div>

  {#if split && secondary}
    {#if collapsed}
      <button
        class="split-expand"
        onclick={() => (collapsed = false)}
        aria-label="Show {collapsedLabel}"
        title="Show {collapsedLabel}"
      >
        <span class="split-expand-icon" aria-hidden="true">»</span>
        <span class="split-expand-text">Show {collapsedLabel}</span>
      </button>
    {:else}
      <!-- Focusable window-splitter separator (WAI-ARIA pattern) -->
      <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
      <div
        class="split-divider"
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize {collapsedLabel}"
        aria-valuemin={MIN_PCT}
        aria-valuemax={MAX_PCT}
        aria-valuenow={Math.round(ui.splitPct)}
        tabindex="0"
        onmousedown={onDividerMousedown}
        onkeydown={onDividerKeydown}
      >
        <button
          class="split-collapse"
          onmousedown={(e) => e.stopPropagation()}
          onclick={() => (collapsed = true)}
          aria-label="Hide {collapsedLabel}"
          title="Hide {collapsedLabel}"
        >
          <span aria-hidden="true">«</span>
        </button>
      </div>
    {/if}

    {@render secondary()}
  {/if}
</div>

<style>
  .split {
    flex: 1;
    display: flex;
    overflow: hidden;
    min-height: 0;
  }
  .split.dragging {
    cursor: col-resize;
    user-select: none;
  }

  .split-list {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: flex 0.18s ease;
    min-width: 0;
  }
  .dragging .split-list {
    transition: none;
  }

  /* ── Divider: 8px hit area, 1px visual line ──────────────────────────────── */

  .split-divider {
    width: 8px;
    flex-shrink: 0;
    position: relative;
    cursor: col-resize;
    display: flex;
    justify-content: center;
  }
  .split-divider::before {
    content: "";
    width: 1px;
    height: 100%;
    background: var(--border);
    transition:
      background 0.15s,
      width 0.15s;
  }
  .split-divider:hover::before,
  .split-divider:focus-visible::before,
  .dragging .split-divider::before {
    width: 3px;
    background: var(--accent);
  }
  .split-divider:focus-visible {
    outline: none;
  }

  .split-collapse {
    position: absolute;
    top: 8px;
    left: 50%;
    transform: translateX(-50%);
    width: 20px;
    height: 20px;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 4px;
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    line-height: 1;
    cursor: pointer;
    z-index: 10;
    transition:
      color 0.15s,
      border-color 0.15s;
  }
  .split-collapse:hover {
    color: var(--accent);
    border-color: var(--accent);
  }

  /* ── Collapsed rail ──────────────────────────────────────────────────────── */

  .split-expand {
    flex-shrink: 0;
    width: 24px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 10px 0;
    background: var(--bg-panel);
    border: none;
    border-right: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: var(--fs-xs);
    cursor: pointer;
    transition:
      color 0.15s,
      background 0.15s;
  }
  .split-expand:hover {
    color: var(--accent);
    background: var(--bg-row-hover);
  }
  .split-expand-icon {
    font-size: var(--fs-sm);
    line-height: 1;
  }
  .split-expand-text {
    writing-mode: vertical-rl;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
</style>
