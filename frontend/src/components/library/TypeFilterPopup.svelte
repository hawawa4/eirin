<script lang="ts">
  import { onMount } from "svelte";
  import type { FrameType } from "../../lib/types";
  import { FRAME_TYPE_META } from "../../lib/types";

  interface Props {
    x: number;
    y: number;
    selected: FrameType[];
    ontoggle: (type: FrameType) => void;
    onclose: () => void;
  }

  let { x, y, selected, ontoggle, onclose }: Props = $props();

  let el: HTMLDivElement;

  onMount(() => {
    el.querySelector<HTMLInputElement>("input")?.focus();
    function onDoc(e: MouseEvent) {
      const t = e.target as Element;
      if (el.contains(t) || t.closest?.("[data-type-filter-toggle]")) return;
      onclose();
    }
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      e.preventDefault();
      e.stopPropagation();
      onclose();
    }
    document.addEventListener("mousedown", onDoc);
    window.addEventListener("keydown", onKey, true);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      window.removeEventListener("keydown", onKey, true);
    };
  });
</script>

<div
  bind:this={el}
  class="type-filter-popup"
  role="group"
  aria-label="Frame type filter"
  style="left: {x}px; top: {y}px"
>
  {#each Object.entries(FRAME_TYPE_META) as [type, meta] (type)}
    <label class="filter-popup-item">
      <input
        type="checkbox"
        checked={selected.includes(type as FrameType)}
        onchange={() => ontoggle(type as FrameType)}
      />
      <span style="color:{meta.color}">{meta.label}</span>
    </label>
  {/each}
</div>

<style>
  .type-filter-popup {
    position: fixed;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 4px 0;
    z-index: 300;
    min-width: 130px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.5);
  }

  .filter-popup-item {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 12px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
  }
  .filter-popup-item:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .filter-popup-item input {
    accent-color: var(--accent);
    cursor: pointer;
  }
</style>
