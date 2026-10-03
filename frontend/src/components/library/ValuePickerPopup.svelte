<script lang="ts">
  import { onMount, tick } from "svelte";
  import type { ValueCount } from "../../lib/library/filters";

  interface Props {
    x: number;
    y: number;
    minWidth: number;
    label: string;
    options: ValueCount[];
    /** The exact value currently picked, if any. */
    selected: string | null;
    onpick: (value: string | null) => void;
    onclose: () => void;
  }

  let { x, y, minWidth, label, options, selected, onpick, onclose }: Props = $props();

  let el: HTMLDivElement;
  let listEl = $state<HTMLUListElement | null>(null);
  let query = $state("");
  let cursor = $state(-1);

  let shown = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return q ? options.filter((o) => o.value.toLowerCase().includes(q)) : options;
  });

  // Start on the picked value; reset to the top match while narrowing.
  $effect(() => {
    void query;
    cursor = query ? 0 : shown.findIndex((o) => o.value === selected);
  });

  async function scrollCursorIntoView() {
    await tick();
    listEl?.querySelector<HTMLElement>(`[data-index="${cursor}"]`)?.scrollIntoView({
      block: "nearest",
    });
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (shown.length === 0) return;
      const d = e.key === "ArrowDown" ? 1 : -1;
      cursor = (Math.max(cursor, d > 0 ? -1 : 0) + d + shown.length) % shown.length;
      scrollCursorIntoView();
    } else if (e.key === "Enter") {
      e.preventDefault();
      const o = shown[cursor];
      if (o) onpick(o.value);
    }
  }

  onMount(() => {
    scrollCursorIntoView();
    function onDoc(e: MouseEvent) {
      const t = e.target as Element;
      if (el.contains(t) || t.closest?.("[data-value-picker-toggle]")) return;
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
  class="value-picker"
  role="dialog"
  aria-label="{label} values"
  style="left: {x}px; top: {y}px; min-width: {minWidth}px"
>
  <!-- svelte-ignore a11y_autofocus -->
  <input
    class="vp-search"
    type="search"
    placeholder="Find {label.toLowerCase()}…"
    aria-label="Find {label.toLowerCase()}"
    spellcheck="false"
    autofocus
    bind:value={query}
    onkeydown={onKeydown}
  />
  {#if selected !== null}
    <button class="vp-clear" onclick={() => onpick(null)}>✕ Any {label.toLowerCase()}</button>
  {/if}
  {#if shown.length === 0}
    <p class="vp-empty">{options.length === 0 ? "No values in this view" : "No match"}</p>
  {:else}
    <ul class="vp-list" role="listbox" aria-label={label} bind:this={listEl}>
      {#each shown as o, i (o.value)}
        <li
          role="option"
          aria-selected={o.value === selected}
          data-index={i}
          class:cursor={i === cursor}
          class:selected={o.value === selected}
          onmousedown={(e) => e.preventDefault()}
          onclick={() => onpick(o.value)}
          onkeydown={() => {}}
          onmouseenter={() => (cursor = i)}
        >
          <span class="vp-value">{o.value}</span>
          <span class="vp-count">{o.count}</span>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .value-picker {
    position: fixed;
    display: flex;
    flex-direction: column;
    max-width: 360px;
    max-height: min(360px, calc(100vh - 16px));
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    z-index: 300;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.5);
    overflow: hidden;
  }

  .vp-search {
    margin: 6px;
    font-size: var(--fs-sm);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-primary);
    padding: 3px 6px;
    outline: none;
  }
  .vp-search:focus {
    border-color: var(--accent);
  }
  .vp-search::placeholder {
    color: var(--text-dim);
  }

  .vp-clear {
    margin: 0 6px 4px;
    padding: 3px 6px;
    font-size: var(--fs-xs);
    text-align: left;
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-secondary);
    cursor: pointer;
  }
  .vp-clear:hover {
    border-color: var(--accent);
    color: var(--text-primary);
  }

  .vp-list {
    list-style: none;
    margin: 0;
    padding: 0 0 4px;
    overflow-y: auto;
  }

  .vp-list li {
    display: flex;
    align-items: baseline;
    gap: 12px;
    padding: 4px 10px;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    cursor: pointer;
    user-select: none;
  }
  .vp-list li.cursor {
    background: var(--bg-row-hover);
  }
  .vp-list li.selected {
    color: var(--accent);
    font-weight: 600;
  }

  .vp-value {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .vp-count {
    flex-shrink: 0;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }

  .vp-empty {
    margin: 0;
    padding: 4px 10px 8px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
</style>
