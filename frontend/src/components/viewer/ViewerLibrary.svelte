<script lang="ts">
  // Library for the read-only server viewer: browse the final images (stacked,
  // processed, raster) one at a time. The desktop's LibraryView is the
  // culling/processing tool; this is just for looking.
  import { onMount, tick } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { GetLibraryFrames } from "$app";
  import type * as app from "$models/app";
  import SplitPane from "../SplitPane.svelte";
  import PreviewPane from "../PreviewPane.svelte";
  import { libraryFrameToEntry, formatDate } from "../../lib/utils";
  import { frameTypeMeta } from "../../lib/library/groups";

  interface Props {
    rootFolder: string;
    /** True while the tab is visible; keyboard navigation only works then. */
    active: boolean;
    /** Opens the Sky Atlas centred on a frame. */
    onshowonatlas?: (nasPath: string) => void;
  }

  let { rootFolder, active, onshowonatlas }: Props = $props();

  const TYPES = ["processed", "stacked", "image"] as const;
  type ViewerType = (typeof TYPES)[number];

  let frames = $state.raw<app.LibraryFrame[]>([]);
  let loading = $state(true);
  let error = $state("");
  let query = $state("");
  let shownTypes = $state<Record<ViewerType, boolean>>({
    processed: true,
    stacked: true,
    image: true,
  });
  let selectedPath = $state<string | null>(null);
  let collapsed = $state(false);
  let listEl = $state<HTMLElement | null>(null);
  // A focus request that arrived before the frames loaded.
  let pendingFocus: string | null = null;

  let typeCounts = $derived.by(() => {
    const c: Record<string, number> = {};
    for (const f of frames) c[f.frameType] = (c[f.frameType] ?? 0) + 1;
    return c;
  });

  /** Filtered frames, by object then newest first: the order prev/next walks. */
  let visible = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return frames
      .filter((f) => shownTypes[f.frameType as ViewerType] ?? true)
      .filter(
        (f) => !q || f.object.toLowerCase().includes(q) || f.fileName.toLowerCase().includes(q),
      )
      .sort(
        (a, b) =>
          objectLabel(a).localeCompare(objectLabel(b), undefined, { numeric: true }) ||
          b.dateObs.localeCompare(a.dateObs),
      );
  });

  let groups = $derived.by(() => {
    const out: { object: string; frames: app.LibraryFrame[] }[] = [];
    for (const f of visible) {
      const label = objectLabel(f);
      const last = out[out.length - 1];
      if (last && last.object === label) last.frames.push(f);
      else out.push({ object: label, frames: [f] });
    }
    return out;
  });

  let selected = $derived(frames.find((f) => f.nasPath === selectedPath) ?? null);
  let selectedIndex = $derived(visible.findIndex((f) => f.nasPath === selectedPath));
  let prevPath = $derived(selectedIndex > 0 ? visible[selectedIndex - 1].nasPath : null);
  let nextPath = $derived(
    selectedIndex >= 0 && selectedIndex < visible.length - 1
      ? visible[selectedIndex + 1].nasPath
      : null,
  );
  let onAtlas = $derived(!!selected && selected.ra !== 0 && selected.pixelScale !== 0);

  function objectLabel(f: app.LibraryFrame): string {
    return f.object || "Unknown object";
  }

  async function select(path: string | null) {
    selectedPath = path;
    if (!path) return;
    await tick();
    listEl
      ?.querySelector(`[data-path="${CSS.escape(path)}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }

  export async function reload(): Promise<void> {
    try {
      frames = (await GetLibraryFrames(rootFolder)) ?? [];
      error = "";
      if (selectedPath && !frames.some((f) => f.nasPath === selectedPath)) selectedPath = null;
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
    }
    if (pendingFocus) focusFile(pendingFocus);
  }

  /** Shows one frame, clearing filters that would hide it (Sky Atlas "Open in Library"). */
  export function focusFile(nasPath: string): void {
    if (loading) {
      pendingFocus = nasPath;
      return;
    }
    pendingFocus = null;
    const f = frames.find((x) => x.nasPath === nasPath);
    if (!f) return;
    query = "";
    if (f.frameType in shownTypes) shownTypes[f.frameType as ViewerType] = true;
    collapsed = false;
    void select(nasPath);
  }

  function onKeydown(e: KeyboardEvent) {
    if (!active || e.ctrlKey || e.metaKey || e.altKey) return;
    const t = e.target as HTMLElement | null;
    if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable)) return;
    if (e.key === "ArrowDown" || e.key === "ArrowRight" || e.key === "j") {
      const next = selectedIndex < 0 ? visible[0]?.nasPath : nextPath;
      if (next) void select(next);
    } else if (e.key === "ArrowUp" || e.key === "ArrowLeft" || e.key === "k") {
      if (prevPath) void select(prevPath);
    } else if (e.key === "Escape" && selectedPath) {
      selectedPath = null;
    } else {
      return;
    }
    e.preventDefault();
  }

  onMount(() => {
    void reload();
    // The server swaps in a new snapshot without a page reload.
    return Events.On("library:updated", () => void reload());
  });
</script>

<svelte:window onkeydown={onKeydown} />

<SplitPane showSecondary={!!selected} collapsedLabel="images" bind:collapsed>
  {#snippet list()}
    <div class="viewer-list">
      <div class="toolbar">
        <input
          class="search"
          type="search"
          placeholder="Search object or file name…"
          aria-label="Search images"
          bind:value={query}
        />
        <div class="chips" role="group" aria-label="Image types">
          {#each TYPES as t (t)}
            {@const meta = frameTypeMeta(t)}
            <button
              class="chip"
              class:on={shownTypes[t]}
              aria-pressed={shownTypes[t]}
              style:--chip-color={meta.color}
              style:--chip-bg={meta.bg}
              onclick={() => (shownTypes[t] = !shownTypes[t])}
            >
              {meta.label} <span class="count">{typeCounts[t] ?? 0}</span>
            </button>
          {/each}
        </div>
      </div>

      {#if loading}
        <div class="status" role="status">Loading images…</div>
      {:else if error}
        <div class="status error" role="alert">
          <p>Couldn't load the library: {error}</p>
          <button class="btn-secondary" onclick={() => reload()}>Retry</button>
        </div>
      {:else if frames.length === 0}
        <div class="status">
          <p class="status-title">No images yet</p>
          <p>
            This viewer shows the stacked and processed images from the library the desktop app
            publishes. They'll appear here once it has published a snapshot.
          </p>
        </div>
      {:else if visible.length === 0}
        <div class="status">No images match.</div>
      {:else}
        <div class="groups" bind:this={listEl}>
          {#each groups as g (g.object)}
            <section class="group">
              <h3 class="group-title">
                {g.object} <span class="count">{g.frames.length}</span>
              </h3>
              {#each g.frames as f (f.nasPath)}
                {@const meta = frameTypeMeta(f.frameType)}
                <button
                  class="row"
                  class:selected={f.nasPath === selectedPath}
                  data-path={f.nasPath}
                  aria-current={f.nasPath === selectedPath ? "true" : undefined}
                  onclick={() => select(f.nasPath)}
                >
                  <span class="badge" style:color={meta.color} style:background={meta.bg}
                    >{meta.short}</span
                  >
                  <span class="name" title={f.fileName}>{f.fileName}</span>
                  <span class="meta">
                    {#if f.filter}<span>{f.filter}</span>{/if}
                    {#if f.dateObs}<span>{formatDate(f.dateObs)}</span>{/if}
                  </span>
                </button>
              {/each}
            </section>
          {/each}
        </div>
      {/if}
    </div>
  {/snippet}

  {#snippet secondary()}
    {#if selected}
      <PreviewPane
        entry={libraryFrameToEntry(selected)}
        qualityFrame={selected}
        onclose={() => (selectedPath = null)}
        onprev={prevPath ? () => select(prevPath) : undefined}
        onnext={nextPath ? () => select(nextPath) : undefined}
        onshowonatlas={onAtlas && onshowonatlas ? () => onshowonatlas(selected.nasPath) : undefined}
        prefetch={nextPath}
        serverRendered
      />
    {/if}
  {/snippet}
</SplitPane>

<style>
  .viewer-list {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-base);
  }

  .toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-panel);
  }

  .search {
    flex: 1 1 200px;
    min-width: 0;
    padding: 5px 9px;
    border-radius: 5px;
    border: 1px solid var(--border);
    background: var(--bg-base);
    color: var(--text-primary);
    font-size: var(--fs-sm);
  }
  .search:focus {
    outline: none;
    border-color: var(--accent);
  }

  .chips {
    display: flex;
    gap: 4px;
  }

  .chip {
    padding: 3px 9px;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: transparent;
    color: var(--text-dim);
    font-size: var(--fs-xs);
    cursor: pointer;
  }
  .chip.on {
    color: var(--chip-color);
    background: var(--chip-bg);
    border-color: var(--chip-color);
  }
  .chip .count {
    opacity: 0.7;
    margin-left: 2px;
  }

  .groups {
    flex: 1;
    overflow-y: auto;
    padding-bottom: 12px;
  }

  .group-title {
    position: sticky;
    top: 0;
    z-index: 1;
    margin: 0;
    padding: 6px 12px;
    font-size: var(--fs-sm);
    font-weight: 600;
    color: var(--text-primary);
    background: var(--bg-row-dir);
    border-bottom: 1px solid var(--border);
  }
  .group-title .count {
    font-weight: 400;
    color: var(--text-dim);
    margin-left: 4px;
  }

  .row {
    width: 100%;
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 10px;
    padding: 6px 12px;
    border: none;
    border-bottom: 1px solid var(--border);
    background: var(--bg-row);
    color: var(--text-primary);
    font-size: var(--fs-sm);
    text-align: left;
    cursor: pointer;
  }
  .row:hover {
    background: var(--bg-row-hover);
  }
  .row.selected {
    background: var(--accent-dim);
  }

  .badge {
    font-family: monospace;
    font-size: var(--fs-xs);
    font-weight: 600;
    padding: 1px 6px;
    border-radius: 3px;
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta {
    display: flex;
    gap: 10px;
    color: var(--text-secondary);
    font-size: var(--fs-xs);
    white-space: nowrap;
  }

  .status {
    padding: 32px 24px;
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    text-align: center;
    max-width: 460px;
    margin: 0 auto;
  }
  .status p {
    margin: 0 0 8px;
    line-height: 1.5;
  }
  .status-title {
    color: var(--text-primary);
    font-size: var(--fs-lg);
    font-weight: 500;
  }
  .status.error {
    color: var(--danger);
  }
</style>
