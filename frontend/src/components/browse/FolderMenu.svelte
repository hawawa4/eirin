<script lang="ts" module>
  // Minimal right-click menu for folders in Browse: the file ContextMenu always
  // offers reject/delete/Siril, which make no sense for a directory.

  export interface FolderMenuItem {
    label: string;
    run: () => void;
  }
</script>

<script lang="ts">
  import { onMount } from "svelte";

  interface Props {
    x: number;
    y: number;
    items: FolderMenuItem[];
    onclose: () => void;
  }

  let { x, y, items, onclose }: Props = $props();

  const EDGE = 4;

  let menuEl: HTMLDivElement;
  let left = $state(0);
  let top = $state(0);
  const restoreFocusTo =
    document.activeElement instanceof HTMLElement ? document.activeElement : null;

  $effect(() => {
    const { width, height } = menuEl.getBoundingClientRect();
    left = x + width > window.innerWidth - EDGE ? Math.max(EDGE, x - width) : x;
    top = y + height > window.innerHeight - EDGE ? Math.max(EDGE, y - height) : y;
  });

  function buttons(): HTMLButtonElement[] {
    return Array.from(menuEl.querySelectorAll<HTMLButtonElement>("button.ctx-item"));
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
  {#each items as item (item.label)}
    <button
      class="ctx-item"
      role="menuitem"
      onclick={() => {
        item.run();
        onclose();
      }}
    >
      {item.label}
    </button>
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
    min-width: 160px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.55);
  }
  .ctx-menu:focus {
    outline: none;
  }

  .ctx-item {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    padding: 5px 14px;
    font-size: var(--fs-sm);
    cursor: pointer;
  }
  .ctx-item:hover,
  .ctx-item:focus-visible {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .ctx-item:focus-visible {
    outline: none;
    box-shadow: inset 2px 0 0 var(--accent);
  }
</style>
