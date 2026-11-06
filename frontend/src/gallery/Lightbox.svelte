<script lang="ts">
  import { onMount } from "svelte";
  import { fade } from "svelte/transition";
  import { levelFor, previewUrl } from "../lib/images";
  import { describeImage } from "./describe";
  import type { GalleryImage } from "./types";

  interface Props {
    images: GalleryImage[];
    /** Position of the open picture in `images`. */
    index: number;
    /** The atlas has pictures, so "Show on the sky" can be offered for solved ones. */
    hasSky: boolean;
    onclose: () => void;
    onselect: (path: string) => void;
    onshowonsky: (path: string) => void;
  }

  let { images, index, hasSky, onclose, onselect, onshowonsky }: Props = $props();

  let img = $derived(images[index]);
  let info = $derived(describeImage(img));
  let prev = $derived(index > 0 ? images[index - 1] : null);
  let next = $derived(index < images.length - 1 ? images[index + 1] : null);

  const THUMB_LEVEL = levelFor(320);
  let viewportW = $state(window.innerWidth);
  let viewportH = $state(window.innerHeight);
  /** Sharp on this screen: the longest side the picture can be drawn at. */
  let level = $derived(levelFor(Math.max(viewportW, viewportH)));

  let loaded = $state(false);
  let failed = $state(false);
  // A new picture starts blurry (its card's thumbnail, already cached) until
  // the sharp one has been decoded.
  $effect(() => {
    void img.path;
    loaded = false;
    failed = false;
  });

  // The neighbours are fetched ahead, so stepping through is instant.
  $effect(() => {
    for (const n of [prev, next]) {
      if (n) new Image().src = previewUrl(n.path, level, 0);
    }
  });

  let dialog: HTMLElement;
  let closeBtn: HTMLButtonElement;
  let opener: Element | null = null;
  onMount(() => {
    opener = document.activeElement;
    closeBtn.focus();
    // The page behind must not scroll under the picture.
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = overflow;
      if (opener instanceof HTMLElement) opener.focus();
    };
  });

  function step(to: GalleryImage | null) {
    if (to) onselect(to.path);
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    switch (e.key) {
      case "Escape":
        onclose();
        break;
      case "ArrowLeft":
        step(prev);
        break;
      case "ArrowRight":
        step(next);
        break;
      case "Tab": {
        // Keep focus inside the dialog.
        const items = [...dialog.querySelectorAll<HTMLElement>("button:not(:disabled)")];
        const first = items[0];
        const last = items[items.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          last.focus();
          break;
        }
        if (!e.shiftKey && document.activeElement === last) {
          first.focus();
          break;
        }
        return;
      }
      default:
        return;
    }
    e.preventDefault();
  }

  // Swipe sideways to step on touch screens.
  let touchX: number | null = null;
  function onTouchStart(e: TouchEvent) {
    touchX = e.touches.length === 1 ? e.touches[0].clientX : null;
  }
  function onTouchEnd(e: TouchEvent) {
    if (touchX === null) return;
    const dx = e.changedTouches[0].clientX - touchX;
    touchX = null;
    if (Math.abs(dx) > 60) step(dx > 0 ? prev : next);
  }
</script>

<svelte:window onkeydown={onKeydown} bind:innerWidth={viewportW} bind:innerHeight={viewportH} />

<div
  class="lightbox"
  role="dialog"
  aria-modal="true"
  aria-label={img.title}
  tabindex="-1"
  bind:this={dialog}
  transition:fade={{ duration: 140 }}
>
  <!-- Clicking the dark around the picture closes it. -->
  <div class="backdrop" onclick={onclose} role="presentation"></div>

  <div
    class="stage"
    ontouchstart={onTouchStart}
    ontouchend={onTouchEnd}
    onclick={(e) => e.target === e.currentTarget && onclose()}
    role="presentation"
  >
    <img
      class="thumb"
      src={previewUrl(img.path, THUMB_LEVEL, 0)}
      alt=""
      aria-hidden="true"
      draggable="false"
    />
    {#key img.path + level}
      <img
        class="full"
        class:loaded
        src={previewUrl(img.path, level, 0)}
        alt={img.title}
        draggable="false"
        onload={() => (loaded = true)}
        onerror={() => (failed = true)}
      />
    {/key}
    {#if failed}<p class="error" role="alert">Couldn't load this picture.</p>{/if}
  </div>

  <button class="nav prev" aria-label="Previous picture" disabled={!prev} onclick={() => step(prev)}
    >‹</button
  >
  <button class="nav next" aria-label="Next picture" disabled={!next} onclick={() => step(next)}
    >›</button
  >
  <button
    class="close"
    aria-label="Close"
    title="Close (Esc)"
    bind:this={closeBtn}
    onclick={onclose}>✕</button
  >

  <div class="info">
    <p class="title">{info.title}</p>
    {#if info.rows.length > 0}
      <dl>
        {#each info.rows as [label, value] (label)}
          <div>
            <dt>{label}</dt>
            <dd>{value}</dd>
          </div>
        {/each}
      </dl>
    {/if}
    {#if hasSky && img.solved}
      <button class="sky" onclick={() => onshowonsky(img.path)}>Show on the sky →</button>
    {/if}
    <p class="count">{index + 1} / {images.length}</p>
  </div>
</div>

<style>
  .lightbox {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: rgb(0 0 0 / 0.96);
    color: #e8e8ea;
  }
  .backdrop {
    position: absolute;
    inset: 0;
  }

  .stage {
    position: absolute;
    /* Room for the info bar below on short screens, the arrows on the sides. */
    inset: 56px 0 0 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 clamp(8px, 7vw, 90px) 150px;
  }
  .stage img {
    position: absolute;
    max-width: calc(100% - 2 * clamp(8px, 7vw, 90px));
    max-height: calc(100% - 150px);
    object-fit: contain;
    user-select: none;
    -webkit-user-drag: none;
  }
  .thumb {
    filter: blur(3px);
  }
  .full {
    opacity: 0;
    transition: opacity 0.25s;
  }
  .full.loaded {
    opacity: 1;
  }
  .error {
    position: relative;
    color: var(--danger);
  }

  .nav,
  .close {
    position: absolute;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid rgb(255 255 255 / 0.18);
    background: rgb(0 0 0 / 0.5);
    color: #fff;
    cursor: pointer;
    font: inherit;
    transition:
      background 0.15s,
      opacity 0.15s;
  }
  .nav:hover:not(:disabled),
  .close:hover {
    background: rgb(255 255 255 / 0.16);
  }
  .nav:disabled {
    opacity: 0.25;
    cursor: default;
  }
  .nav {
    top: calc(50% - 75px);
    width: 44px;
    height: 64px;
    margin-top: -32px;
    border-radius: 8px;
    font-size: 2rem;
    line-height: 1;
  }
  .prev {
    left: clamp(6px, 1.5vw, 20px);
  }
  .next {
    right: clamp(6px, 1.5vw, 20px);
  }
  .close {
    top: 12px;
    right: 14px;
    width: 38px;
    height: 38px;
    border-radius: 50%;
    font-size: var(--fs-lg);
  }
  .nav:focus-visible,
  .close:focus-visible,
  .sky:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }

  .info {
    position: absolute;
    inset: auto 0 0 0;
    z-index: 1;
    padding: 18px clamp(14px, 4vw, 40px) calc(16px + env(safe-area-inset-bottom, 0px));
    background: linear-gradient(to top, rgb(0 0 0 / 0.92) 55%, transparent);
    text-align: center;
    pointer-events: none;
  }
  .info > * {
    pointer-events: auto;
  }
  .title {
    font-size: var(--fs-xl);
    font-weight: 600;
  }
  dl {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 2px 22px;
    margin-top: 6px;
    font-size: var(--fs-sm);
  }
  dl > div {
    display: flex;
    gap: 6px;
  }
  dt {
    color: #a8a8b0;
  }
  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }
  .sky {
    margin-top: 10px;
    padding: 5px 14px;
    border-radius: 6px;
    border: 1px solid var(--accent);
    background: none;
    color: var(--accent);
    font: inherit;
    font-size: var(--fs-sm);
    cursor: pointer;
  }
  .sky:hover {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .count {
    margin-top: 8px;
    font-size: var(--fs-xs);
    color: #8a8a92;
  }

  @media (max-width: 560px) {
    .nav {
      width: 36px;
      height: 52px;
      font-size: 1.6rem;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .full {
      transition: none;
    }
  }
</style>
