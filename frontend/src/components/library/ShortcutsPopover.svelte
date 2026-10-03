<script lang="ts">
  import { onMount } from "svelte";

  interface Props {
    onclose: () => void;
  }

  let { onclose }: Props = $props();

  const SHORTCUTS: [keys: string[], action: string][] = [
    [["↑", "↓"], "Previous / next frame (also k / j)"],
    [["Click"], "Preview (click again to close)"],
    [["Ctrl", "Click"], "Toggle selection"],
    [["Shift", "Click"], "Select range"],
    [["Space"], "Toggle checkbox of current frame"],
    [["Ctrl", "A"], "Select all frames in view"],
    [["x"], "Reject current / selected, then advance"],
    [["u"], "Restore current / selected"],
    [["Delete"], "Delete from disk (asks first)"],
    [["b"], "Blink selected frames"],
    [["Esc"], "Close preview, then clear selection"],
    [["?"], "Show / hide this help"],
  ];

  let el: HTMLDivElement;

  onMount(() => {
    function onDoc(e: MouseEvent) {
      const t = e.target as Element;
      if (el.contains(t) || t.closest?.("[data-shortcuts-toggle]")) return;
      onclose();
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  });
</script>

<div bind:this={el} class="shortcuts" role="dialog" aria-label="Keyboard shortcuts">
  <div class="sc-head">
    <span class="sc-title">Keyboard shortcuts</span>
    <button class="sc-close" onclick={onclose} aria-label="Close shortcuts">✕</button>
  </div>
  <dl class="sc-list">
    {#each SHORTCUTS as [keys, action] (action)}
      <dt>
        {#each keys as k, i (i)}{#if i > 0}<span class="plus">+</span>{/if}<kbd>{k}</kbd>{/each}
      </dt>
      <dd>{action}</dd>
    {/each}
  </dl>
</div>

<style>
  .shortcuts {
    position: absolute;
    top: 8px;
    right: 12px;
    z-index: 250;
    width: min(360px, 90vw);
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 6px;
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.55);
    padding: 10px 14px 12px;
  }
  .sc-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;
  }
  .sc-title {
    font-size: var(--fs-md);
    font-weight: 600;
    color: var(--text-primary);
  }
  .sc-close {
    width: 28px;
    height: 28px;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: var(--fs-sm);
  }
  .sc-close:hover {
    color: var(--text-primary);
    background: var(--bg-row-hover);
  }
  .sc-list {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 5px 12px;
    align-items: center;
    margin: 0;
  }
  dt {
    white-space: nowrap;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  dd {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--text-primary);
  }
  kbd {
    display: inline-block;
    min-width: 20px;
    padding: 1px 5px;
    border: 1px solid var(--border-accent);
    border-radius: 3px;
    background: var(--bg-base);
    color: var(--text-primary);
    font-family: inherit;
    font-size: var(--fs-xs);
    text-align: center;
  }
  .plus {
    margin: 0 2px;
  }
</style>
