<script lang="ts" generics="G extends NamedGroup">
  import { tick } from "svelte";
  import type * as app from "$models/app";
  import { filterGroups, searchCatalog, type NamedGroup } from "../../lib/atlas/objects";

  interface Props {
    groups: G[];
    catalog: app.CatalogObject[];
    activeName: string | null;
    /** `additive` is true for Shift/Ctrl/Cmd+click. */
    onselectgroup: (g: G, additive: boolean) => void;
    onselectcatalog: (obj: app.CatalogObject) => void;
  }

  let { groups, catalog, activeName, onselectgroup, onselectcatalog }: Props = $props();

  let open = $state(true);
  let search = $state("");
  let searchEl = $state<HTMLInputElement | null>(null);

  let shownGroups = $derived(filterGroups(groups, search));
  let catalogMatches = $derived(searchCatalog(catalog, search));

  /** Opens the browser (if collapsed) and focuses the search box. */
  export async function focusSearch() {
    open = true;
    await tick();
    searchEl?.focus();
    searchEl?.select();
  }

  function onSearchKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.stopPropagation();
      if (search) search = "";
      else searchEl?.blur();
    } else if (e.key === "Enter") {
      if (shownGroups.length > 0) onselectgroup(shownGroups[0], false);
      else if (catalogMatches.length > 0) onselectcatalog(catalogMatches[0]);
    }
  }
</script>

<div class="obj-browser" class:collapsed={!open}>
  <button
    class="obj-header"
    onclick={() => (open = !open)}
    aria-expanded={open}
    title={open ? "Collapse object list" : "Expand object list"}
  >
    <span class="obj-title">Objects</span>
    <span class="obj-count">{groups.length}</span>
    <span class="obj-chevron" aria-hidden="true">{open ? "▴" : "▾"}</span>
  </button>
  {#if open}
    <div class="obj-body">
      <input
        bind:this={searchEl}
        class="obj-search"
        type="search"
        placeholder="Search objects or sky catalog…  ( / )"
        aria-label="Search objects and sky catalog"
        bind:value={search}
        onkeydown={onSearchKeydown}
      />
      <div class="obj-scroll">
        {#if search.trim()}
          <div class="obj-section">Your frames</div>
        {/if}
        <ul class="obj-list">
          {#each shownGroups as g (g.name)}
            <li>
              <button
                class="obj-item"
                class:active={g.name === activeName}
                aria-current={g.name === activeName ? "true" : undefined}
                onclick={(e) => onselectgroup(g, e.shiftKey || e.ctrlKey || e.metaKey)}
                title="Show {g.name} ({g.count} frame{g.count === 1 ? '' : 's'})"
              >
                <span class="obj-item-name">{g.name}</span>
                <span class="obj-item-count">{g.count}</span>
              </button>
            </li>
          {:else}
            <li class="obj-empty">No matching frames</li>
          {/each}
        </ul>
        {#if search.trim()}
          <div class="obj-section">Sky catalog</div>
          <ul class="obj-list">
            {#each catalogMatches as obj (obj.name + obj.ra)}
              <li>
                <button
                  class="obj-item"
                  onclick={() => onselectcatalog(obj)}
                  title="Fly to {obj.name}"
                >
                  <span
                    class="obj-kind"
                    class:dso={obj.type !== "star"}
                    aria-label={obj.type === "star" ? "Star" : "Deep-sky object"}
                  ></span>
                  <span class="obj-item-name">{obj.name}</span>
                </button>
              </li>
            {:else}
              <li class="obj-empty">No catalog matches</li>
            {/each}
          </ul>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .obj-browser {
    position: absolute;
    top: 44px; /* below the HUD */
    left: 10px;
    width: 230px;
    background: color-mix(in srgb, var(--bg-panel) 94%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 6px;
    z-index: 20;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    max-height: calc(100% - 140px);
    backdrop-filter: blur(3px);
    box-shadow: 0 4px 18px rgba(0, 0, 0, 0.35);
  }
  .obj-browser.collapsed {
    max-height: none;
  }
  .obj-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    background: none;
    border: none;
    cursor: pointer;
    width: 100%;
    color: var(--text-primary);
    font-size: var(--fs-sm);
    font-weight: 600;
  }
  .obj-header:hover {
    background: var(--bg-row-hover);
  }
  .obj-title {
    flex: 1;
    text-align: left;
  }
  .obj-count {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
    background: var(--bg-row);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0 6px;
  }
  .obj-chevron {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .obj-body {
    display: flex;
    flex-direction: column;
    border-top: 1px solid var(--border);
    overflow: hidden;
    min-height: 0;
  }
  .obj-search {
    margin: 8px 8px 6px;
    padding: 5px 8px;
    font-size: var(--fs-sm);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    outline: none;
    flex-shrink: 0;
  }
  .obj-search::placeholder {
    color: var(--text-dim);
  }
  .obj-search:focus {
    border-color: var(--accent);
  }
  .obj-scroll {
    overflow-y: auto;
    min-height: 0;
    padding-bottom: 4px;
  }
  .obj-section {
    padding: 6px 10px 2px;
    font-size: var(--fs-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-secondary);
  }
  .obj-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  .obj-item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    background: none;
    border: none;
    border-left: 3px solid transparent;
    padding: 4px 10px 4px 7px;
    cursor: pointer;
    text-align: left;
  }
  .obj-item:hover {
    background: var(--bg-row-hover);
  }
  .obj-item.active {
    background: var(--bg-row-hover);
    border-left-color: var(--accent);
  }
  .obj-item.active .obj-item-name {
    color: var(--accent);
    font-weight: 600;
  }
  .obj-item-name {
    flex: 1;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .obj-item-count {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
    flex-shrink: 0;
  }
  .obj-kind {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--atlas-star);
    flex-shrink: 0;
  }
  .obj-kind.dso {
    background: none;
    border: 1.5px solid var(--atlas-dso);
  }
  .obj-empty {
    padding: 4px 10px 6px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-style: italic;
  }
</style>
