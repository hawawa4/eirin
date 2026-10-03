<script lang="ts">
  import { onMount } from "svelte";
  import type { CtxEntry, CtxMenuState } from "../lib/types";
  import { FRAME_TYPE_META } from "../lib/types";

  interface Props {
    menu: CtxMenuState;
    onclose: () => void;
    onreject: (entry: CtxEntry) => void;
    onrestore: (entry: CtxEntry) => void;
    onharddelete: (entry: CtxEntry) => void;
    onopensiril: (entry: CtxEntry) => void;
    /** Hides the "Set type" section when absent. */
    onchangetype?: (entry: CtxEntry, newType: string) => void;
    onrename?: (entry: CtxEntry) => void;
    oneditmeta?: (entry: CtxEntry) => void;
    oncreateproject?: () => void;
    /** "Show in folder" — hidden when absent. */
    onreveal?: (entry: CtxEntry) => void;
    /** "Copy path" — hidden when absent. */
    oncopypath?: (entry: CtxEntry) => void;
  }

  let {
    menu,
    onclose,
    onreject,
    onrestore,
    onharddelete,
    onopensiril,
    onchangetype,
    onrename,
    oneditmeta,
    oncreateproject,
    onreveal,
    oncopypath,
  }: Props = $props();

  const EDGE = 4;

  let menuEl: HTMLDivElement;
  let left = $state(0);
  let top = $state(0);

  const single = $derived(menu.selectionCount <= 1);
  const restoreFocusTo =
    document.activeElement instanceof HTMLElement ? document.activeElement : null;

  // ── Positioning: clamp into the viewport, flipping up/left on overflow ────
  $effect(() => {
    const { x, y } = menu;
    const { width, height } = menuEl.getBoundingClientRect();
    left = x + width > window.innerWidth - EDGE ? Math.max(EDGE, x - width) : x;
    top = y + height > window.innerHeight - EDGE ? Math.max(EDGE, y - height) : y;
  });

  // ── Dismissal: outside click, Escape, window blur/scroll/resize ───────────
  onMount(() => {
    items()[0]?.focus();

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

  // ── Keyboard navigation ───────────────────────────────────────────────────
  function items(): HTMLButtonElement[] {
    return Array.from(menuEl.querySelectorAll<HTMLButtonElement>("button.ctx-item:not(:disabled)"));
  }

  function onMenuKeydown(e: KeyboardEvent) {
    const list = items();
    if (list.length === 0) return;
    const i = list.indexOf(document.activeElement as HTMLButtonElement);
    let next: number | null = null;
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

  function run(fn: () => void) {
    fn();
    onclose();
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
  <button
    class="ctx-item ctx-siril"
    role="menuitem"
    disabled={!menu.sirilAvailable}
    onclick={() => onopensiril(menu.entry)}
    title={menu.sirilAvailable
      ? "Open in Siril"
      : menu.selectionCount > 1
        ? "Only available for a single file"
        : "Siril not found — configure it in Settings"}
  >
    Open with Siril
  </button>
  {#if single && (onreveal || oncopypath)}
    {#if onreveal}
      <button class="ctx-item" role="menuitem" onclick={() => run(() => onreveal?.(menu.entry))}>
        Show in folder
      </button>
    {/if}
    {#if oncopypath}
      <button class="ctx-item" role="menuitem" onclick={() => run(() => oncopypath?.(menu.entry))}>
        Copy path
      </button>
    {/if}
  {/if}
  {#if single && onchangetype}
    <div class="ctx-sep" role="separator"></div>
    <div role="group" aria-labelledby="ctx-type-label">
      <span class="ctx-label" id="ctx-type-label">Set type</span>
      {#each Object.entries(FRAME_TYPE_META) as [type, meta] (type)}
        <button
          class="ctx-item ctx-type-item"
          class:ctx-type-current={menu.entry.frameType === type}
          role="menuitemradio"
          aria-checked={menu.entry.frameType === type}
          onclick={() => run(() => onchangetype?.(menu.entry, type))}
        >
          <span class="ctx-type-dot" style="color:{meta.color}" aria-hidden="true">●</span>
          {meta.label}
          {#if menu.entry.frameType === type}<span class="ctx-type-check" aria-hidden="true">✓</span
            >{/if}
        </button>
      {/each}
    </div>
  {/if}
  {#if single && (onrename || oneditmeta)}
    <div class="ctx-sep" role="separator"></div>
    {#if onrename}
      <button class="ctx-item" role="menuitem" onclick={() => run(() => onrename?.(menu.entry))}>
        Rename…
      </button>
    {/if}
    {#if oneditmeta}
      <button class="ctx-item" role="menuitem" onclick={() => run(() => oneditmeta?.(menu.entry))}>
        Edit Metadata…
      </button>
    {/if}
  {/if}
  <div class="ctx-sep" role="separator"></div>
  {#if menu.entry.isRejected}
    <button class="ctx-item" role="menuitem" onclick={() => onrestore(menu.entry)}>
      {menu.selectionCount > 1 ? `Restore ${menu.selectionCount} frames` : "Restore"}
    </button>
  {:else}
    <button class="ctx-item" role="menuitem" onclick={() => onreject(menu.entry)}>
      {menu.selectionCount > 1 ? `Reject ${menu.selectionCount} frames` : "Reject"}
    </button>
  {/if}
  {#if oncreateproject}
    <div class="ctx-sep" role="separator"></div>
    <button
      class="ctx-item ctx-create-project"
      role="menuitem"
      onclick={() => run(() => oncreateproject?.())}
    >
      {menu.selectionCount > 1
        ? `Create project (${menu.selectionCount} frames)…`
        : "Create project…"}
    </button>
  {/if}
  <div class="ctx-sep" role="separator"></div>
  <button class="ctx-item ctx-danger" role="menuitem" onclick={() => onharddelete(menu.entry)}>
    {menu.selectionCount > 1 ? `Hard Delete ${menu.selectionCount} frames…` : "Hard Delete…"}
  </button>
</div>

<style>
  .ctx-menu {
    position: fixed;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 3px 0;
    z-index: 1000;
    min-width: 180px;
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
    transition:
      background 0.1s,
      color 0.1s;
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

  .ctx-danger,
  .ctx-danger:hover:not(:disabled),
  .ctx-danger:focus-visible {
    color: var(--danger);
  }
  .ctx-danger:hover:not(:disabled),
  .ctx-danger:focus-visible {
    background: color-mix(in srgb, var(--danger) 15%, transparent);
  }

  .ctx-siril,
  .ctx-siril:hover:not(:disabled),
  .ctx-siril:focus-visible,
  .ctx-create-project,
  .ctx-create-project:hover:not(:disabled),
  .ctx-create-project:focus-visible {
    color: var(--accent);
  }

  .ctx-sep {
    height: 1px;
    background: var(--border);
    margin: 3px 0;
  }

  .ctx-label {
    display: block;
    padding: 4px 14px 2px;
    font-size: var(--fs-xs);
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }

  .ctx-type-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding-top: 4px;
    padding-bottom: 4px;
  }

  .ctx-type-dot {
    font-size: 0.55rem;
    flex-shrink: 0;
  }

  .ctx-type-current {
    color: var(--text-primary);
    font-weight: 600;
  }

  .ctx-type-check {
    margin-left: auto;
    color: var(--accent);
    font-size: var(--fs-sm);
  }
</style>
