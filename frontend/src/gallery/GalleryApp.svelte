<script lang="ts">
  import { onMount, tick, untrack } from "svelte";
  import { Events } from "@wailsio/runtime";
  import type * as app from "$models/app";
  import SkyAtlas from "../components/SkyAtlas.svelte";
  import SlideshowView from "../components/SlideshowView.svelte";
  import { exitImmersive, immersive } from "../lib/shell/immersive.svelte";
  import { usePrefsLocally } from "../lib/prefs";
  import { getGallery } from "./api";
  import { setActiveFormat } from "./appShim";
  import { describeImage } from "./describe";
  import { hashFor, parseHash, type Route } from "./route";
  import type { GalleryFormat, GalleryInfo, GalleryView } from "./types";
  import GalleryHeader from "./GalleryHeader.svelte";
  import GalleryGrid from "./GalleryGrid.svelte";
  import Lightbox from "./Lightbox.svelte";

  // The gallery is read-only and shared: nothing a visitor changes is saved
  // anywhere but their own browser.
  usePrefsLocally(true);

  let info = $state<GalleryInfo | null>(null);
  let loadError = $state("");

  let route = $state<Route>(parseHash(location.hash));
  let view = $derived<GalleryView>(route.view ?? info?.view ?? "grid");
  /** Views stay mounted (hidden) once visited, so they keep their state. */
  let visited = $state<Partial<Record<GalleryView, boolean>>>({});
  $effect(() => {
    visited[view] = true;
  });

  let skyAtlas = $state<{ focusFrame: (nasPath: string) => void } | null>(null);

  // ── Loading ───────────────────────────────────────────────────────────────
  async function load() {
    try {
      info = await getGallery();
      loadError = "";
    } catch (e) {
      loadError = String(e);
    }
  }

  onMount(() => {
    void load();
    // The server emits this when it has loaded a newer library snapshot.
    return Events.On("library:updated", () => void load());
  });

  $effect(() => {
    if (!info) return;
    document.title = info.title;
    if (info.theme === "blue") document.documentElement.removeAttribute("data-theme");
    else document.documentElement.setAttribute("data-theme", info.theme);
  });

  // ── Navigation ────────────────────────────────────────────────────────────
  function onHashChange() {
    route = parseHash(location.hash);
  }

  function go(next: GalleryView) {
    if (next === view && !route.image) return;
    location.hash = hashFor(next);
  }

  function openImage(path: string) {
    location.hash = hashFor("grid", path);
  }

  /** Closing leaves the picture's link out of the history rather than adding to it. */
  function closeImage() {
    history.replaceState(null, "", hashFor("grid"));
    route = parseHash(location.hash);
  }

  async function showOnSky(path: string) {
    location.hash = hashFor("atlas");
    await tick();
    skyAtlas?.focusFrame(path);
  }

  // Fullscreen belongs to the view that asked for it (slideshow / atlas tour).
  $effect(() => {
    void view;
    untrack(exitImmersive);
  });

  // ── Lookups ───────────────────────────────────────────────────────────────
  let byPath = $derived(
    new Map((info?.images ?? []).concat(info?.sky ?? []).map((i) => [i.path, i])),
  );

  // The server sends both renderings of every picture; the page shows one at a
  // time. The visitor's choice wins, else the server's default — unless that
  // format has nothing to show.
  let chosen = $state<GalleryFormat | null>(null);
  let counts = $derived({
    fits: info?.images.filter((i) => i.format === "fits").length ?? 0,
    raster: info?.images.filter((i) => i.format === "raster").length ?? 0,
  });
  let format = $derived.by<GalleryFormat>(() => {
    const want = chosen ?? info?.defaultFormat ?? "fits";
    if (counts[want] === 0 && counts[want === "fits" ? "raster" : "fits"] > 0) {
      return want === "fits" ? "raster" : "fits";
    }
    return want;
  });
  // Before the Atlas and the Slideshow (re)load: they ask the shim for pictures.
  $effect.pre(() => setActiveFormat(format));

  let visible = $derived({
    images: info?.images.filter((i) => i.format === format) ?? [],
    sky: info?.sky.filter((i) => i.format === format) ?? [],
  });
  let hasSky = $derived(visible.sky.length > 0);
  let openIndex = $derived(
    route.image ? visible.images.findIndex((i) => i.path === route.image) : -1,
  );

  // A link to a picture in the other format opens it by switching to that format.
  $effect(() => {
    const img = route.image ? byPath.get(route.image) : undefined;
    if (img && img.format !== format && info?.images.some((i) => i.path === img.path)) {
      chosen = img.format;
    }
  });

  function describe(frame: app.LibraryFrame) {
    const img = byPath.get(frame.nasPath);
    return img ? describeImage(img) : { title: frame.object, rows: [] };
  }
</script>

<svelte:window onhashchange={onHashChange} />

<div class="layout">
  {#if info && !immersive.on}
    <GalleryHeader
      {info}
      {view}
      {hasSky}
      onnavigate={go}
      {format}
      {counts}
      onformat={(f) => (chosen = f)}
    />
  {/if}

  <main class="views">
    {#if !info}
      <div class="status" role={loadError ? "alert" : "status"}>
        {#if loadError}
          <p class="status-title">The gallery couldn't be loaded</p>
          <p class="status-detail">{loadError}</p>
          <button class="status-btn" onclick={load}>Try again</button>
        {:else}
          <div class="spinner" aria-hidden="true"></div>
          <p class="status-title">Loading…</p>
        {/if}
      </div>
    {:else if !info.loaded}
      <div class="status" role="status">
        <div class="spinner" aria-hidden="true"></div>
        <p class="status-title">The gallery is being prepared</p>
        <p class="status-detail">It will appear here as soon as it's ready.</p>
      </div>
    {:else}
      <div class="view" class:hidden={view !== "grid"}>
        <GalleryGrid {info} images={visible.images} onopen={openImage} />
      </div>

      {#if visited.atlas}
        <div class="view" class:hidden={view !== "atlas"}>
          {#key format}
            <SkyAtlas
              bind:this={skyAtlas}
              rootPath="gallery"
              active={view === "atlas"}
              theme={info.theme}
              gallery
            />
          {/key}
        </div>
      {/if}

      {#if visited.slideshow}
        <div class="view" class:hidden={view !== "slideshow"}>
          {#key format}
            <SlideshowView
              rootPath="gallery"
              active={view === "slideshow"}
              gallery
              {describe}
              onshowonatlas={hasSky ? showOnSky : undefined}
            />
          {/key}
        </div>
      {/if}
    {/if}
  </main>
</div>

{#if info && openIndex >= 0}
  <Lightbox
    images={visible.images}
    index={openIndex}
    {hasSky}
    onclose={closeImage}
    onselect={openImage}
    onshowonsky={showOnSky}
  />
{/if}

<style>
  /* The app's own styles fix the page to the window; mobile browsers' toolbars
     make 100vh taller than what's visible, so use the dynamic height. */
  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
    height: 100dvh;
  }

  .views {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .view {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
    overflow: hidden;
  }
  .view.hidden {
    display: none;
  }

  .status {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 24px;
    text-align: center;
  }
  .status-title {
    font-size: var(--fs-xl);
    font-weight: 500;
  }
  .status-detail {
    color: var(--text-secondary);
    max-width: 420px;
    user-select: text;
  }
  .status-btn {
    margin-top: 6px;
    padding: 7px 18px;
    border-radius: 6px;
    border: 1px solid var(--accent);
    background: var(--accent);
    color: var(--accent-contrast);
    font: inherit;
    cursor: pointer;
  }
  .spinner {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    border: 2px solid var(--border-accent);
    border-top-color: var(--accent);
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation-duration: 3s;
    }
  }
</style>
