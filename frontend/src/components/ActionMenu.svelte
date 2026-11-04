<script lang="ts">
  // Generic right-click menu for things that aren't a single library frame
  // (groups, storage tiles, projects, atlas images…). Library frame rows use
  // ContextMenu.svelte. Item type and shared builders live in lib/shell/menu.ts.
  import { onMount } from "svelte";
  import { tidy, type ActionMenuItem } from "../lib/shell/menu";

  interface Props {
    x: number;
    y: number;
    items: ActionMenuItem[];
    onclose: () => void;
  }

  let { x, y, items: rawItems, onclose }: Props = $props();
  let items = $derived(tidy(rawItems));

  const EDGE = 4;

  let menuEl: HTMLDivElement;
  let left = $state(0);
  let top = $state(0);

  const restoreFocusTo =
    document.activeElement instanceof HTMLElement ? document.activeElement : null;

  // Clamp into the viewport, flipping up/left on overflow.
  $effect(() => {
    const { width, height } = menuEl.getBoundingClientRect();
    left = x + width > window.innerWidth - EDGE ? Math.max(EDGE, x - width) : x;
    top = y + height > window.innerHeight - EDGE ? Math.max(EDGE, y - height) : y;
  });

  function buttons(): HTMLButtonElement[] {
    return Array.from(menuEl.querySelectorAll<HTMLButtonElement>("button.ctx-item:not(:disabled)"));
  }

  onMount(() => {
    buttons()[0]?.focus();

    function onDocMousedown(e: MouseEvent) {
      if (!menuEl.contains(e.target as Node)) onclose();
    }
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      e.preventDefault();
      e.stopPropagation();
      if (restoreFocusTo?.isConnected) restoreFocusTo.focus();
      onclose();
    }
    function onScroll(e: Event) {
      if (e.target instanceof Node && menuEl.contains(e.target)) return;
      onclose();
    }
    document.addEventListener("mousedown", onDocMousedown);
    window.addEventListener("keydown", onKey, true);
    window.addEventListener("blur", onclose);
    window.addEventListener("resize", onclose);
    window.addEventListener("scroll", onScroll, true);
    return () => {
      document.removeEventListener("mousedown", onDocMousedown);
      window.removeEventListener("keydown", onKey, true);
      window.removeEventListener("blur", onclose);
      window.removeEventListener("resize", onclose);
      window.removeEventListener("scroll", onScroll, true);
    };
  });

  function onMenuKeydown(e: KeyboardEvent) {
    const list = buttons();
    if (list.length === 0) return;
    const i = list.indexOf(document.activeElement as HTMLButtonElement);
    let next: number;
    switch (e.key) {
      case "ArrowDown":
        next = i < 0 ? 0 : (i + 1) % list.length;
        break;
      case "ArrowUp":
        next = i < 0 ? list.length - 1 : (i - 1 + list.length) % list.length;
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = list.length - 1;
        break;
      case "Tab":
        e.preventDefault();
        onclose();
        return;
      default:
        return;
    }
    e.preventDefault();
    list[next].focus();
  }
</script>

<div
  bind:this={menuEl}
  class="ctx-menu"
  style="left: {left}px; top: {top}px"
  role="menu"
  tabindex="-1"
  onkeydown={onMenuKeydown}
  oncontextmenu={(e) => e.preventDefault()}
>
  {#each items as item, i (i)}
    {#if "separator" in item}
      <div class="ctx-sep" role="separator"></div>
    {:else if "heading" in item}
      <span class="ctx-heading">{item.heading}</span>
    {:else}
      <button
        class="ctx-item"
        class:danger={item.danger}
        class:primary={item.primary}
        role="menuitem"
        disabled={!!item.disabledReason}
        title={item.disabledReason}
        onclick={() => {
          onclose();
          item.run();
        }}
      >
        <span class="ctx-text">{item.label}</span>
        {#if item.hint}<kbd class="ctx-hint">{item.hint}</kbd>{/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .ctx-menu {
    position: fixed;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 3px 0;
    z-index: 1000;
    min-width: 200px;
    max-width: 360px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.55);
  }
  .ctx-menu:focus {
    outline: none;
  }

  .ctx-heading {
    display: block;
    padding: 5px 14px 3px;
    font-size: var(--fs-xs);
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.07em;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ctx-item {
    display: flex;
    align-items: center;
    gap: 16px;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    padding: 5px 14px;
    font-size: var(--fs-sm);
    cursor: pointer;
  }
  .ctx-text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ctx-item:hover:not(:disabled),
  .ctx-item:focus-visible {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .ctx-item:focus-visible {
    outline: none;
    box-shadow: inset 2px 0 0 var(--accent);
  }
  .ctx-item:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .ctx-item.primary,
  .ctx-item.primary:hover:not(:disabled) {
    color: var(--accent);
  }
  .ctx-item.danger,
  .ctx-item.danger:hover:not(:disabled),
  .ctx-item.danger:focus-visible {
    color: var(--danger);
  }
  .ctx-item.danger:hover:not(:disabled),
  .ctx-item.danger:focus-visible {
    background: color-mix(in srgb, var(--danger) 15%, transparent);
  }

  .ctx-hint {
    flex-shrink: 0;
    font-family: inherit;
    font-size: var(--fs-xs);
    color: var(--text-dim);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 0 5px;
  }

  .ctx-sep {
    height: 1px;
    background: var(--border);
    margin: 3px 0;
  }
</style>
