<script lang="ts">
  import { REEL_INTERVALS_S, type Reel } from "../lib/reel.svelte";

  interface Props {
    reel: Reel<unknown>;
    fullscreen: boolean;
    onfullscreen: () => void;
    /** Adds a button that turns pictures to fit the screen's orientation. */
    onrotation?: () => void;
    rotation?: boolean;
    /** Adds a stop button (the atlas tour is entered and left from here). */
    onstop?: () => void;
    /** Fade the bar out after the pointer has been still for a moment. */
    autohide?: boolean;
    /** Whether the bar is currently shown; the parent hides the cursor with it. */
    visible?: boolean;
    /** Adds a FITS / PNG switch (the slideshow never mixes the two). */
    format?: {
      value: "fits" | "png";
      fitsCount: number;
      pngCount: number;
      onchange: (f: "fits" | "png") => void;
    };
  }

  let {
    reel,
    fullscreen,
    onfullscreen,
    onstop,
    onrotation,
    rotation = false,
    autohide = false,
    visible = $bindable(true),
    format,
  }: Props = $props();

  const HIDE_AFTER_MS = 2500;
  let hideTimer: ReturnType<typeof setTimeout> | undefined;
  let hovering = false;

  function poke() {
    visible = true;
    clearTimeout(hideTimer);
    if (autohide && !hovering) hideTimer = setTimeout(() => (visible = false), HIDE_AFTER_MS);
  }

  $effect(() => {
    void autohide;
    poke();
    return () => clearTimeout(hideTimer);
  });
</script>

<svelte:window onmousemove={poke} onmousedown={poke} />

<div
  class="reel-controls"
  class:hidden={!visible}
  role="group"
  aria-label="Playback"
  onmouseenter={() => {
    hovering = true;
    poke();
  }}
  onmouseleave={() => {
    hovering = false;
    poke();
  }}
>
  <button class="rc-btn" title="Previous (←)" aria-label="Previous" onclick={() => reel.prev()}
    >⏮</button
  >
  <button
    class="rc-btn"
    title={reel.playing ? "Pause (Space)" : "Play (Space)"}
    aria-label={reel.playing ? "Pause" : "Play"}
    onclick={() => reel.toggle()}>{reel.playing ? "⏸" : "▶"}</button
  >
  <button class="rc-btn" title="Next (→)" aria-label="Next" onclick={() => reel.next()}>⏭</button>
  <span class="rc-count" aria-label="Position">{reel.position + 1} / {reel.length}</span>
  <select
    class="rc-select"
    title="Time per image"
    aria-label="Time per image"
    value={reel.intervalMs / 1000}
    onchange={(e) => reel.setIntervalMs(Number(e.currentTarget.value) * 1000)}
  >
    {#each REEL_INTERVALS_S as s (s)}
      <option value={s}>{s} s</option>
    {/each}
  </select>
  {#if format}
    <div class="rc-seg" role="group" aria-label="Image format">
      <button
        class="rc-seg-btn"
        class:active={format.value === "fits"}
        aria-pressed={format.value === "fits"}
        disabled={format.fitsCount === 0}
        title="Processed FITS images (P to switch)"
        onclick={() => format.onchange("fits")}>FITS {format.fitsCount}</button
      >
      <button
        class="rc-seg-btn"
        class:active={format.value === "png"}
        aria-pressed={format.value === "png"}
        disabled={format.pngCount === 0}
        title="Exported PNG, JPEG and TIFF images (P to switch)"
        onclick={() => format.onchange("png")}>PNG {format.pngCount}</button
      >
    </div>
  {/if}
  {#if onrotation}
    <button
      class="rc-btn"
      class:active={rotation}
      title="Turn pictures to fit the screen (R)"
      aria-label="Turn pictures to fit the screen"
      aria-pressed={rotation}
      onclick={onrotation}>⟳</button
    >
  {/if}
  <button
    class="rc-btn"
    class:active={fullscreen}
    title={fullscreen ? "Leave fullscreen (F)" : "Fullscreen (F)"}
    aria-label={fullscreen ? "Leave fullscreen" : "Fullscreen"}
    aria-pressed={fullscreen}
    onclick={onfullscreen}>⛶</button
  >
  {#if onstop}
    <button class="rc-btn" title="Stop (Esc)" aria-label="Stop" onclick={onstop}>✕</button>
  {/if}
</div>

<style>
  .reel-controls {
    position: absolute;
    bottom: 18px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 6px;
    background: color-mix(in srgb, var(--bg-panel) 90%, transparent);
    border: 1px solid var(--border);
    border-radius: 8px;
    backdrop-filter: blur(4px);
    z-index: 50;
    transition: opacity 0.35s;
  }
  .reel-controls.hidden {
    opacity: 0;
    pointer-events: none;
  }

  .rc-btn {
    width: 32px;
    height: 30px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    border-radius: 5px;
    color: var(--text-primary);
    font-size: var(--fs-md);
    cursor: pointer;
  }
  .rc-btn:hover {
    background: var(--bg-row-hover);
    color: var(--accent);
  }
  .rc-btn.active {
    background: var(--accent);
    color: var(--accent-contrast);
  }

  .rc-count {
    min-width: 54px;
    text-align: center;
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
    color: var(--text-secondary);
  }

  .rc-seg {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 5px;
    overflow: hidden;
  }
  .rc-seg-btn {
    font-size: var(--fs-xs);
    padding: 4px 8px;
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-variant-numeric: tabular-nums;
  }
  .rc-seg-btn + .rc-seg-btn {
    border-left: 1px solid var(--border);
  }
  .rc-seg-btn:hover:not(:disabled):not(.active) {
    color: var(--text-primary);
  }
  .rc-seg-btn.active {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .rc-seg-btn:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .rc-select {
    font-size: var(--fs-xs);
    color: var(--text-primary);
    background-color: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 4px 22px 4px 6px;
    margin-right: 2px;
  }

  @media (prefers-reduced-motion: reduce) {
    .reel-controls {
      transition: none;
    }
  }
</style>
