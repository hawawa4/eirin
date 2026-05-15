<script lang="ts">
  import type { CtxEntry, CtxMenuState } from "../lib/types";
  import { FRAME_TYPE_META } from "../lib/types";

  interface Props {
    menu: CtxMenuState;
    onclose: () => void;
    onreject: (entry: CtxEntry) => void;
    onrestore: (entry: CtxEntry) => void;
    onharddelete: (entry: CtxEntry) => void;
    onopensiril: (entry: CtxEntry) => void;
    onchangetype: (entry: CtxEntry, newType: string) => void;
  }

  let { menu, onclose, onreject, onrestore, onharddelete, onopensiril, onchangetype }: Props = $props();

  $effect(() => {
    function onDoc(e: MouseEvent) {
      const el = document.getElementById("ctx-menu");
      if (el && !el.contains(e.target as Node)) onclose();
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  });
</script>

<div id="ctx-menu" class="ctx-menu" style="left: {menu.x}px; top: {menu.y}px">
  <button
    class="ctx-item ctx-siril"
    class:ctx-disabled={!menu.sirilAvailable}
    disabled={!menu.sirilAvailable}
    onclick={() => onopensiril(menu.entry)}
    title={menu.sirilAvailable ? "Open in Siril" : "Siril not found — configure it in Settings"}
  >
    Open with Siril
  </button>
  <div class="ctx-sep"></div>
  <span class="ctx-label">Set type</span>
  {#each Object.entries(FRAME_TYPE_META) as [type, meta]}
    <button
      class="ctx-item ctx-type-item"
      class:ctx-type-current={menu.entry.frameType === type}
      onclick={() => { onchangetype(menu.entry, type); onclose(); }}
    >
      <span class="ctx-type-dot" style="color:{meta.color}">●</span>
      {meta.label}
      {#if menu.entry.frameType === type}<span class="ctx-type-check">✓</span>{/if}
    </button>
  {/each}
  <div class="ctx-sep"></div>
  {#if menu.entry.isRejected}
    <button class="ctx-item" onclick={() => onrestore(menu.entry)}>Restore</button>
  {:else}
    <button class="ctx-item" onclick={() => onreject(menu.entry)}>Reject</button>
  {/if}
  <div class="ctx-sep"></div>
  <button class="ctx-item ctx-danger" onclick={() => onharddelete(menu.entry)}>Hard Delete…</button>
</div>

<style>
  :global(.ctx-menu) {
    position: fixed;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 3px 0;
    z-index: 1000;
    min-width: 160px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.55);
  }

  :global(.ctx-item) {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    padding: 5px 14px;
    font-size: 0.82rem;
    cursor: pointer;
    transition:
      background 0.1s,
      color 0.1s;
  }
  :global(.ctx-item:hover) {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  :global(.ctx-item.ctx-danger) {
    color: var(--danger);
  }
  :global(.ctx-item.ctx-danger:hover) {
    background: #2a1020;
    color: var(--danger);
  }
  :global(.ctx-item.ctx-siril) {
    color: var(--accent);
  }
  :global(.ctx-item.ctx-siril:hover) {
    background: var(--bg-row-hover);
    color: var(--accent);
  }
  :global(.ctx-item.ctx-disabled) {
    opacity: 0.4;
    cursor: not-allowed;
  }
  :global(.ctx-item.ctx-disabled:hover) {
    background: transparent;
  }

  :global(.ctx-sep) {
    height: 1px;
    background: var(--border);
    margin: 3px 0;
  }

  :global(.ctx-label) {
    display: block;
    padding: 3px 14px 1px;
    font-size: 0.68rem;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.07em;
    opacity: 0.65;
  }

  :global(.ctx-type-item) {
    display: flex !important;
    align-items: center;
    gap: 6px;
    padding-top: 4px !important;
    padding-bottom: 4px !important;
  }

  :global(.ctx-type-dot) {
    font-size: 0.55rem;
    flex-shrink: 0;
  }

  :global(.ctx-type-current) {
    background: var(--accent-dim);
    color: var(--text-primary) !important;
  }

  :global(.ctx-type-check) {
    margin-left: auto;
    color: var(--accent);
    font-size: 0.8rem;
  }
</style>
