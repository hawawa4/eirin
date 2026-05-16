<script lang="ts">
  import { onDestroy } from "svelte";
  import type { app } from "../../wailsjs/go/models";
  import { GeneratePreview } from "../../wailsjs/go/app/App.js";

  interface Props {
    frames: app.LibraryFrame[];
    onclose: () => void;
  }

  let { frames, onclose }: Props = $props();

  // ── State ─────────────────────────────────────────────────────────────────
  let currentIndex = $state(0);
  let intervalMs = $state(500);
  let playing = $state(false);
  let previews = $state<(string | null)[]>(frames.map(() => null));
  let loadingCount = $state(0);
  let stretchLevel = $state(2);

  let timerId: ReturnType<typeof setInterval> | null = null;

  // Preload all preview images on mount
  $effect(() => {
    loadingCount = frames.length;
    frames.forEach((f, i) => {
      GeneratePreview(f.nasPath, stretchLevel)
        .then((url) => {
          previews = previews.map((p, j) => (j === i ? url : p));
          loadingCount--;
        })
        .catch(() => {
          previews = previews.map((p, j) => (j === i ? "" : p));
          loadingCount--;
        });
    });
  });

  function startBlink() {
    if (timerId) return;
    timerId = setInterval(() => {
      currentIndex = (currentIndex + 1) % frames.length;
    }, intervalMs);
    playing = true;
  }

  function stopBlink() {
    if (timerId) { clearInterval(timerId); timerId = null; }
    playing = false;
  }

  function togglePlay() { playing ? stopBlink() : startBlink(); }

  function step(dir: -1 | 1) {
    stopBlink();
    currentIndex = (currentIndex + dir + frames.length) % frames.length;
  }

  function onSpeedChange(e: Event) {
    intervalMs = parseInt((e.target as HTMLInputElement).value);
    if (playing) { stopBlink(); startBlink(); }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") onclose();
    if (e.key === " ") { e.preventDefault(); togglePlay(); }
    if (e.key === "ArrowRight") step(1);
    if (e.key === "ArrowLeft")  step(-1);
  }

  onDestroy(() => stopBlink());

  const current = $derived(frames[currentIndex]);
  const currentPreview = $derived(previews[currentIndex]);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="blink-backdrop" onmousedown={(e) => { if (e.target === e.currentTarget) onclose(); }}>
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div class="blink-modal" role="dialog" onkeydown={onKeydown}>
    <div class="blink-header">
      <span class="blink-title">Blink Comparison — {frames.length} frames</span>
      <button class="blink-close" onclick={onclose}>✕</button>
    </div>

    <div class="blink-viewport">
      {#if loadingCount > 0 && !currentPreview}
        <div class="blink-loading">Loading previews… ({frames.length - loadingCount}/{frames.length})</div>
      {:else if currentPreview}
        <img src={currentPreview} alt={current?.fileName ?? ""} class="blink-img" draggable="false" />
      {:else}
        <div class="blink-loading">Preview unavailable</div>
      {/if}
      <div class="blink-badge">{currentIndex + 1} / {frames.length}</div>
    </div>

    <div class="blink-info">
      <span class="blink-name" title={current?.nasPath}>{current?.fileName}</span>
      {#if current?.dateObs}
        <span class="blink-date">{current.dateObs.slice(0, 16).replace("T", " ")}</span>
      {/if}
      {#if current?.fwhm && current.qualityAnalyzed}
        <span class="blink-stat">FWHM {current.fwhm.toFixed(2)}{current.fwhmUnit || "px"}</span>
      {/if}
    </div>

    <div class="blink-controls">
      <button class="blink-btn" onclick={() => step(-1)} title="Previous (←)">◀</button>
      <button class="blink-btn blink-play" class:playing onclick={togglePlay} title="Play/Pause (Space)">
        {playing ? "⏸" : "▶"}
      </button>
      <button class="blink-btn" onclick={() => step(1)} title="Next (→)">▶</button>

      <div class="blink-speed">
        <span class="blink-speed-label">Speed</span>
        <input
          type="range"
          min="100"
          max="2000"
          step="100"
          value={intervalMs}
          oninput={onSpeedChange}
          class="blink-slider"
        />
        <span class="blink-speed-val">{intervalMs}ms</span>
      </div>

      <div class="blink-dots">
        {#each frames as _, i}
          <button
            class="blink-dot"
            class:active={i === currentIndex}
            onclick={() => { stopBlink(); currentIndex = i; }}
            title={frames[i]?.fileName ?? ""}
          ></button>
        {/each}
      </div>
    </div>
  </div>
</div>

<style>
  .blink-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.72);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 500;
  }

  .blink-modal {
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    width: min(90vw, 900px);
    max-height: 90vh;
    box-shadow: 0 16px 48px rgba(0,0,0,0.7);
    overflow: hidden;
    outline: none;
  }

  .blink-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .blink-title { font-size: 0.85rem; font-weight: 600; color: var(--text-primary); }
  .blink-close { background: transparent; border: none; color: var(--text-secondary); font-size: 1rem; cursor: pointer; padding: 2px 6px; border-radius: 4px; }
  .blink-close:hover { background: var(--bg-row-hover); color: var(--text-primary); }

  .blink-viewport {
    flex: 1;
    min-height: 0;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #08090f;
    overflow: hidden;
  }

  .blink-img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    display: block;
    user-select: none;
  }

  .blink-loading {
    color: var(--text-secondary);
    font-size: 0.85rem;
  }

  .blink-badge {
    position: absolute;
    top: 8px;
    left: 8px;
    background: rgba(0,0,0,0.6);
    color: var(--text-secondary);
    font-size: 0.72rem;
    padding: 2px 7px;
    border-radius: 3px;
    font-variant-numeric: tabular-nums;
  }

  .blink-info {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 5px 14px;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .blink-name { font-size: 0.8rem; font-family: "Consolas", monospace; color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
  .blink-date { font-size: 0.75rem; color: var(--text-secondary); white-space: nowrap; font-variant-numeric: tabular-nums; }
  .blink-stat { font-size: 0.75rem; color: var(--accent); white-space: nowrap; }

  .blink-controls {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 14px;
    flex-shrink: 0;
    flex-wrap: wrap;
  }

  .blink-btn {
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    font-size: 0.8rem;
    padding: 4px 10px;
    cursor: pointer;
    transition: background 0.1s, color 0.1s;
    min-width: 32px;
  }
  .blink-btn:hover { background: var(--bg-row-hover); color: var(--text-primary); }
  .blink-play { min-width: 44px; font-size: 1rem; }
  .blink-play.playing { color: var(--accent); border-color: var(--accent); }

  .blink-speed {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: 8px;
  }
  .blink-speed-label { font-size: 0.72rem; color: var(--text-secondary); }
  .blink-slider { width: 90px; accent-color: var(--accent); cursor: pointer; }
  .blink-speed-val { font-size: 0.72rem; color: var(--text-secondary); width: 42px; font-variant-numeric: tabular-nums; }

  .blink-dots {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;
    flex-wrap: wrap;
    max-width: 300px;
  }
  .blink-dot {
    width: 8px; height: 8px; border-radius: 50%;
    background: var(--border-accent);
    border: none; cursor: pointer; padding: 0; transition: background 0.1s, transform 0.1s;
  }
  .blink-dot.active { background: var(--accent); transform: scale(1.4); }
  .blink-dot:hover { background: var(--text-secondary); }
</style>
