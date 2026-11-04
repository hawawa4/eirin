<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { fade } from "svelte/transition";
  import { Events } from "@wailsio/runtime";
  import type * as app from "$models/app";
  import { GetLibraryFramesByType } from "$app";
  import { isModalOpen, isTypingTarget } from "../lib/keys";
  import { isFits, isRasterFile } from "../lib/utils";
  import { ui } from "../lib/uiState.svelte";
  import { Reel } from "../lib/reel.svelte";
  import { immersive, exitImmersive, toggleImmersive } from "../lib/shell/immersive.svelte";
  import { levelFor, levelRank, loadImage, previewUrl } from "../lib/images";
  import { describeFrame } from "../lib/slideshow/frameInfo";
  import ReelControls from "./ReelControls.svelte";
  import FrameInfoCard from "./slideshow/FrameInfoCard.svelte";
  import ActionMenu from "./ActionMenu.svelte";
  import { fileMenuItems, type ActionMenuItem } from "../lib/shell/menu";

  interface Props {
    rootPath: string;
    /** True while this view's tab is visible. The reel and shortcuts pause otherwise. */
    active?: boolean;
    /** Desktop app: fullscreen is the native window rather than the browser's. */
    desktopMode?: boolean;
    /** Request a library scan (empty state). */
    onscan?: () => void;
    onopeninlibrary?: (nasPath: string) => void;
    onshowonatlas?: (nasPath: string) => void;
  }

  let {
    rootPath,
    active = true,
    desktopMode = false,
    onscan,
    onopeninlibrary,
    onshowonatlas,
  }: Props = $props();

  // ── Right-click menu ──────────────────────────────────────────────────────
  let menu = $state<{ x: number; y: number; items: ActionMenuItem[] } | null>(null);

  function openMenu(e: MouseEvent) {
    e.preventDefault();
    const f = reel.current;
    if (!f) return;
    const other = format === "fits" ? "PNG" : "FITS";
    const items: ActionMenuItem[] = [
      { heading: f.object || f.fileName },
      {
        label: reel.playing ? "Pause" : "Play",
        hint: "Space",
        primary: true,
        run: () => reel.toggle(),
      },
      { label: "Next", hint: "→", run: () => reel.next() },
      { label: "Previous", hint: "←", run: () => reel.prev() },
      {
        label: `Show ${other} images`,
        hint: "P",
        disabledReason: otherCount === 0 ? `No ${other} images` : undefined,
        run: () => setFormat(format === "fits" ? "png" : "fits"),
      },
      {
        label: immersive.on ? "Leave fullscreen" : "Fullscreen",
        hint: "F",
        run: () => toggleImmersive(desktopMode),
      },
      { separator: true },
      ...fileMenuItems({
        path: f.nasPath,
        desktop: desktopMode,
        onopeninlibrary: onopeninlibrary ? () => onopeninlibrary(f.nasPath) : undefined,
        onshowonatlas:
          onshowonatlas && f.ra !== 0 && f.pixelScale !== 0
            ? () => onshowonatlas(f.nasPath)
            : undefined,
      }),
    ];
    menu = { x: e.clientX, y: e.clientY, items };
  }

  /** Processed images are finished pictures: shown as they are, never stretched. */
  const STRETCH_LEVEL = 0;
  const FADE_MS = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ? 0 : 1400;

  /** Final images by format; the slideshow shows one set, never both. */
  let fitsFrames = $state.raw<app.LibraryFrame[]>([]);
  let pngFrames = $state.raw<app.LibraryFrame[]>([]);
  let format = $derived<"fits" | "png">(
    ui.slideshowFormat ?? (fitsFrames.length > 0 || pngFrames.length === 0 ? "fits" : "png"),
  );
  let frames = $derived(format === "fits" ? fitsFrames : pngFrames);
  let otherCount = $derived(format === "fits" ? pngFrames.length : fitsFrames.length);

  function setFormat(f: "fits" | "png") {
    if (f === format) return;
    ui.slideshowFormat = f;
    reel.index = 0;
  }
  let loading = $state(true);
  let loadError = $state("");
  const reel = new Reel<app.LibraryFrame>(() => frames);

  /** The picture on screen; it only changes once the next one has been fetched. */
  let shown = $state.raw<{
    frame: app.LibraryFrame;
    src: string;
    /** Preview level of src (see lib/images). */
    level: number;
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
      const [processed, images] = await Promise.all([
        GetLibraryFramesByType(rootPath, "processed"),
        GetLibraryFramesByType(rootPath, "image"),
      ]);
      if (seq !== loadSeq) return;
      const keep = reel.current?.nasPath;
      const all = [...(processed ?? []), ...(images ?? [])].filter((f) => !f.isRejected);
      // Processed FITS, or the PNG/JPEG/TIFF exports — the same pictures rendered twice.
      fitsFrames = all
        .filter((f) => f.frameType === "processed" && isFits(f.fileName))
        .sort(byObjectThenName);
      pngFrames = all.filter((f) => isRasterFile(f.fileName)).sort(byObjectThenName);
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
  /**
   * The preview level that fills the slideshow area on this screen (a picture
   * is fitted, so its longest side is at most the area's). Before the area is
   * measured, the screen's size stands in.
   */
  let areaLevel = $derived(
    levelFor(Math.max(areaW, areaH) || Math.max(window.screen.width, window.screen.height)),
  );

  /** Decoded pictures by URL (most recent last), so a prefetched slide shows at once. */
  // eslint-disable-next-line svelte/prefer-svelte-reactivity -- a cache, intentionally non-reactive
  const decoded = new Map<string, Promise<HTMLImageElement>>();
  const DECODED_KEEP = 4;

  function decode(url: string): Promise<HTMLImageElement> {
    let p = decoded.get(url);
    if (p) {
      decoded.delete(url); // re-insert as most recently used
    } else {
      p = loadImage(url);
      p.catch(() => decoded.delete(url)); // a failed load may be retried
    }
    decoded.set(url, p);
    while (decoded.size > DECODED_KEEP) decoded.delete(decoded.keys().next().value!);
    return p;
  }

  let showSeq = 0;
  $effect(() => {
    const frame = reel.current;
    const level = areaLevel;
    if (!frame) {
      shown = null;
      return;
    }
    // The same picture, already sharp enough (e.g. the window got smaller).
    const cur = untrack(() => shown);
    if (cur?.frame.nasPath === frame.nasPath && levelRank(cur.level) >= levelRank(level)) return;
    const seq = ++showSeq;
    const url = previewUrl(frame.nasPath, level, STRETCH_LEVEL);
    untrack(() => {
      // Decode up front: the picture's shape decides its rotation, and a decoded
      // image fades in without a stutter. A sharper version of the picture on
      // screen just replaces it.
      decode(url)
        .then((img) => {
          if (seq !== showSeq) return;
          slideError = "";
          shown = { frame, src: url, level, width: img.naturalWidth, height: img.naturalHeight };
          const next = reel.upcoming;
          if (next) {
            decode(previewUrl(next.nasPath, level, STRETCH_LEVEL)).catch(() => {
              /* best-effort; shown (with its error) when its turn comes */
            });
          }
        })
        .catch((err: unknown) => {
          if (seq !== showSeq) return;
          slideError = `Couldn't load ${frame.fileName}: ${err instanceof Error ? err.message : String(err)}`;
        });
    });
  });

  $effect(() => {
    reel.run(active && !loading);
  });

  // ── Keyboard ──────────────────────────────────────────────────────────────
  function onKeydown(e: KeyboardEvent) {
    if (!active || isTypingTarget(e) || isModalOpen() || menu) return;
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
      case "p":
      case "P":
        if (otherCount === 0) return;
        setFormat(format === "fits" ? "png" : "fits");
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
  oncontextmenu={openMenu}
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
  {:else if frames.length === 0 && otherCount > 0}
    <div class="note">
      <p class="note-title">No {format === "fits" ? "FITS" : "PNG"} images</p>
      <p>
        {otherCount}
        {format === "fits" ? "PNG / JPEG / TIFF export" : "processed FITS image"}{otherCount === 1
          ? ""
          : "s"} can be shown instead.
      </p>
      <button class="note-btn" onclick={() => setFormat(format === "fits" ? "png" : "fits")}
        >Show {format === "fits" ? "PNG" : "FITS"}</button
      >
    </div>
  {:else if frames.length === 0}
    <div class="note">
      <p class="note-title">No processed images yet</p>
      <p>
        The slideshow shows your processed images: files with “processed”, “final” or “hero” in the
        name, and exported PNG / JPEG / TIFF pictures.
      </p>
      {#if onscan}<button class="note-btn" onclick={onscan}>Scan library</button>{/if}
    </div>
  {:else if !shown && !slideError}
    <p class="note" role="status">Loading image…</p>
  {/if}

  {#if slideError}
    <p class="slide-error" role="alert">{slideError}</p>
  {/if}

  {#if frames.length > 0}
    <ReelControls
      format={{
        value: format,
        fitsCount: fitsFrames.length,
        pngCount: pngFrames.length,
        onchange: setFormat,
      }}
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

{#if menu}
  <ActionMenu x={menu.x} y={menu.y} items={menu.items} onclose={() => (menu = null)} />
{/if}

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
  .note-btn {
    margin-top: 10px;
    padding: 6px 16px;
    font-size: var(--fs-sm);
    background: transparent;
    color: #e8e8ea;
    border: 1px solid #55555f;
    border-radius: 6px;
    cursor: pointer;
  }
  .note-btn:hover {
    border-color: #e8e8ea;
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
