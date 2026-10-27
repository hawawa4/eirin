<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { fade } from "svelte/transition";
  import { Events } from "@wailsio/runtime";
  import type * as app from "$models/app";
  import { GetLibraryFramesByType } from "$app";
  import { isModalOpen, isTypingTarget } from "../lib/keys";
  import { isFits } from "../lib/utils";
  import { Reel } from "../lib/reel.svelte";
  import { immersive, exitImmersive, toggleImmersive } from "../lib/shell/immersive.svelte";
  import { loadViewerPreview, prefetchViewerPreview } from "../lib/viewer/previews";
  import { describeFrame } from "../lib/slideshow/frameInfo";
  import ReelControls from "./ReelControls.svelte";
  import FrameInfoCard from "./slideshow/FrameInfoCard.svelte";

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
  let shown = $state.raw<{
    frame: app.LibraryFrame;
    src: string;
    width: number;
    height: number;
  } | null>(null);
  let slideError = $state("");
  let controlsVisible = $state(true);

  // ── Orientation ───────────────────────────────────────────────────────────
  // A portrait picture on a landscape screen (or the other way round) is turned
  // a quarter so it fills the screen. The comparison is against the slideshow
  // area itself rather than a "mobile or desktop" guess: a phone held upright
  // is portrait, so portrait pictures stay as they are there, and rotating the
  // phone (or resizing the window) flips the decision.
  const PREF_ROTATE = "eirin.slideshowRotate";
  /** Pictures this close to square gain nothing from a turn. */
  const SQUARE_RATIO = 1.1;

  function loadRotatePref(): boolean {
    try {
      return localStorage.getItem(PREF_ROTATE) !== "false";
    } catch {
      return true;
    }
  }

  let autoRotate = $state(loadRotatePref());
  let areaW = $state(0);
  let areaH = $state(0);

  function toggleRotate() {
    autoRotate = !autoRotate;
    try {
      localStorage.setItem(PREF_ROTATE, String(autoRotate));
    } catch {
      /* storage unavailable */
    }
  }

  function needsTurn(w: number, h: number): boolean {
    if (!autoRotate || areaW === 0 || areaH === 0 || w === 0 || h === 0) return false;
    if (Math.max(w, h) / Math.min(w, h) < SQUARE_RATIO) return false;
    return w > h !== areaW > areaH;
  }

  let turned = $derived(shown ? needsTurn(shown.width, shown.height) : false);

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
      // Only the processed FITS: the PNG/JPEG/TIFF exports are renderings of those.
      frames = result.filter((f) => !f.isRejected && isFits(f.fileName)).sort(byObjectThenName);
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
        .then(async (p) => {
          // Decode up front: the picture's shape decides its rotation, and a decoded
          // image fades in without a stutter.
          const img = new Image();
          img.src = p.dataUrl;
          await img.decode();
          if (seq !== showSeq) return;
          slideError = "";
          shown = { frame, src: p.dataUrl, width: img.naturalWidth, height: img.naturalHeight };
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
      case "r":
      case "R":
        toggleRotate();
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
  bind:clientWidth={areaW}
  bind:clientHeight={areaH}
  ondblclick={() => toggleImmersive(desktopMode)}
  role="presentation"
>
  {#if shown}
    {#key shown.frame.nasPath}
      <!-- The new picture fades in on top while the old one stays opaque underneath, then drops away. -->
      <img
        class="slide"
        class:turned
        style:--area-w="{areaW}px"
        style:--area-h="{areaH}px"
        src={shown.src}
        alt={shown.frame.object || shown.frame.fileName}
        draggable="false"
        in:fade={{ duration: FADE_MS }}
        out:fade={{ duration: 0, delay: FADE_MS }}
      />
      <div class="info" in:fade={{ duration: FADE_MS }} out:fade={{ duration: FADE_MS / 2 }}>
        <FrameInfoCard info={describeFrame(shown.frame)} />
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
      <p>The slideshow shows every processed FITS image in your library.</p>
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
      rotation={autoRotate}
      onrotation={toggleRotate}
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
    /* Double-clicking (fullscreen) must not select the picture or text, which
       WebKit paints as a blue wash over everything. */
    user-select: none;
    -webkit-user-select: none;
    -webkit-tap-highlight-color: transparent;
  }
  .slideshow :global(*) {
    user-select: none;
    -webkit-user-select: none;
  }
  .slideshow :global(::selection) {
    background: transparent;
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
    -webkit-user-drag: none;
  }
  /* Turned a quarter clockwise: laid out with the area's sides swapped, then rotated into place. */
  .slide.turned {
    inset: auto;
    left: 50%;
    top: 50%;
    width: var(--area-h);
    height: var(--area-w);
    transform: translate(-50%, -50%) rotate(90deg);
  }

  .info {
    position: absolute;
    inset: 0;
    pointer-events: none;
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
