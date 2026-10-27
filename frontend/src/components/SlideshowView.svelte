<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { fade } from "svelte/transition";
  import { Events } from "@wailsio/runtime";
  import type * as app from "$models/app";
  import { GetLibraryFramesByType } from "$app";
  import { isModalOpen, isTypingTarget } from "../lib/keys";
  import { Reel } from "../lib/reel.svelte";
  import { immersive, exitImmersive, toggleImmersive } from "../lib/shell/immersive.svelte";
  import { loadViewerPreview, prefetchViewerPreview } from "../lib/viewer/previews";
  import ReelControls from "./ReelControls.svelte";

  interface Props {
    rootPath: string;
    /** True while this view's tab is visible. The reel and shortcuts pause otherwise. */
    active?: boolean;
    /** Desktop app: fullscreen is the native window rather than the browser's. */
    desktopMode?: boolean;
  }

  let { rootPath, active = true, desktopMode = false }: Props = $props();

  /** Processed images are finished pictures: shown as they are, never stretched. */
  const STRETCH_LEVEL = 0;
  const FADE_MS = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ? 0 : 1400;

  let frames = $state.raw<app.LibraryFrame[]>([]);
  let loading = $state(true);
  let loadError = $state("");
  const reel = new Reel<app.LibraryFrame>(() => frames);

  /** The picture on screen; it only changes once the next one has been fetched. */
  let shown = $state.raw<{ frame: app.LibraryFrame; src: string } | null>(null);
  let slideError = $state("");
  let controlsVisible = $state(true);

  // ── Loading ───────────────────────────────────────────────────────────────
  function byObjectThenName(a: app.LibraryFrame, b: app.LibraryFrame): number {
    return (
      a.object.localeCompare(b.object, undefined, { numeric: true }) ||
      a.fileName.localeCompare(b.fileName, undefined, { numeric: true })
    );
  }

  let loadSeq = 0;
  async function load() {
    const seq = ++loadSeq;
    if (!rootPath) {
      frames = [];
      loading = false;
      return;
    }
    try {
      const result = (await GetLibraryFramesByType(rootPath, "processed")) ?? [];
      if (seq !== loadSeq) return;
      const keep = reel.current?.nasPath;
      frames = result.filter((f) => !f.isRejected).sort(byObjectThenName);
      // A library update shouldn't throw the viewer back to the first picture.
      const i = keep ? frames.findIndex((f) => f.nasPath === keep) : -1;
      reel.index = Math.max(0, i);
      loadError = "";
    } catch (err) {
      if (seq === loadSeq) loadError = String(err);
    } finally {
      if (seq === loadSeq) loading = false;
    }
  }

  onMount(() => {
    void load();
    return Events.On("library:updated", () => void load());
  });

  // ── Showing the current picture ───────────────────────────────────────────
  let showSeq = 0;
  $effect(() => {
    const frame = reel.current;
    if (!frame) {
      shown = null;
      return;
    }
    const seq = ++showSeq;
    untrack(() => {
      loadViewerPreview(frame.nasPath, STRETCH_LEVEL)
        .then((p) => {
          if (seq !== showSeq) return;
          slideError = "";
          shown = { frame, src: p.dataUrl };
          const next = reel.upcoming;
          if (next) prefetchViewerPreview(next.nasPath, STRETCH_LEVEL);
        })
        .catch((err: unknown) => {
          if (seq !== showSeq) return;
          slideError = `Couldn't load ${frame.fileName}: ${String(err)}`;
        });
    });
  });

  $effect(() => {
    reel.run(active && !loading);
  });

  // ── Keyboard ──────────────────────────────────────────────────────────────
  function onKeydown(e: KeyboardEvent) {
    if (!active || isTypingTarget(e) || isModalOpen()) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    switch (e.key) {
      case " ":
        reel.toggle();
        break;
      case "ArrowRight":
        reel.next();
        break;
      case "ArrowLeft":
        reel.prev();
        break;
      case "f":
      case "F":
        toggleImmersive(desktopMode);
        break;
      case "Escape":
        if (!immersive.on) return;
        exitImmersive();
        break;
      default:
        return;
    }
    e.preventDefault();
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div
  class="slideshow"
  class:idle={immersive.on && !controlsVisible}
  ondblclick={() => toggleImmersive(desktopMode)}
  role="presentation"
>
  {#if shown}
    {#key shown.frame.nasPath}
      <!-- The new picture fades in on top while the old one stays opaque underneath, then drops away. -->
      <img
        class="slide"
        src={shown.src}
        alt={shown.frame.object || shown.frame.fileName}
        in:fade={{ duration: FADE_MS }}
        out:fade={{ duration: 0, delay: FADE_MS }}
      />
      <div class="caption" in:fade={{ duration: FADE_MS }} out:fade={{ duration: FADE_MS / 2 }}>
        <span class="cap-object">{shown.frame.object || shown.frame.fileName}</span>
        {#if shown.frame.object}<span class="cap-file">{shown.frame.fileName}</span>{/if}
      </div>
    {/key}
  {/if}

  {#if loading}
    <p class="note" role="status">Loading images…</p>
  {:else if loadError}
    <p class="note error" role="alert">Couldn't load the library: {loadError}</p>
  {:else if frames.length === 0}
    <div class="note">
      <p class="note-title">No processed images yet</p>
      <p>The slideshow shows every image of the <em>processed</em> type in your library.</p>
    </div>
  {:else if !shown && !slideError}
    <p class="note" role="status">Loading image…</p>
  {/if}

  {#if slideError}
    <p class="slide-error" role="alert">{slideError}</p>
  {/if}

  {#if frames.length > 0}
    <ReelControls
      {reel}
      fullscreen={immersive.on}
      onfullscreen={() => toggleImmersive(desktopMode)}
      autohide={immersive.on && active}
      bind:visible={controlsVisible}
    />
  {/if}
</div>

<style>
  .slideshow {
    flex: 1;
    position: relative;
    overflow: hidden;
    background: #000;
    color: #e8e8ea;
  }
  .slideshow.idle {
    cursor: none;
  }

  .slide {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
    user-select: none;
    -webkit-user-drag: none;
  }

  .caption {
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
    padding: 18px 24px 56px;
    display: flex;
    align-items: baseline;
    gap: 12px;
    background: linear-gradient(to bottom, rgb(0 0 0 / 0.65), transparent);
    pointer-events: none;
  }
  .cap-object {
    font-size: var(--fs-xl);
    font-weight: 500;
  }
  .cap-file {
    font-size: var(--fs-sm);
    opacity: 0.6;
  }

  .note {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    font-size: var(--fs-md);
    color: #a8a8b0;
    text-align: center;
    padding: 24px;
  }
  .note-title {
    font-size: var(--fs-xl);
    color: #e8e8ea;
  }
  .note.error {
    color: #e07060;
  }

  .slide-error {
    position: absolute;
    top: 14px;
    left: 50%;
    transform: translateX(-50%);
    font-size: var(--fs-sm);
    background: rgb(0 0 0 / 0.7);
    color: #e07060;
    padding: 5px 12px;
    border-radius: 5px;
  }
</style>
