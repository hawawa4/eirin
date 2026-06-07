<script lang="ts">
  import { onDestroy } from "svelte";
  import type * as app from "$models/app";
  import { GeneratePreview } from "$app";

  interface Props {
    frames: app.LibraryFrame[];
    onclose: () => void;
    onreject?: (nasPath: string) => void;
    onharddelete?: (nasPath: string, name: string) => void;
  }

  let { frames, onclose, onreject, onharddelete }: Props = $props();

  // ── State ─────────────────────────────────────────────────────────────────
  let currentIndex = $state(0);
  let intervalMs = $state(500);
  let playing = $state(false);
  let stretchLevel = $state(2);
  let confirmDelete = $state(false);

  // Cache: nasPath → data-URL. Only a sliding window of entries is kept.
  const cache = new Map<string, string>();
  // Tracks in-flight fetches so we don't double-fetch.
  const fetching = new Set<string>();

  // How many frames to preload ahead and behind the current index.
  const AHEAD = 2;
  const BEHIND = 1;
  const WINDOW = AHEAD + BEHIND + 1;

  let timerId: ReturnType<typeof setInterval> | null = null;

  // Derive the URL for the current frame from the cache (reactive via version bump).
  let cacheVersion = $state(0);
  const currentPreview = $derived(
    (() => {
      void cacheVersion; // depend on version so this re-evaluates after fetches
      return cache.get(frames[currentIndex]?.nasPath ?? "") ?? null;
    })(),
  );
  const currentLoading = $derived(
    (() => {
      void cacheVersion;
      const path = frames[currentIndex]?.nasPath ?? "";
      return !cache.has(path) && fetching.has(path);
    })(),
  );

  // Whenever currentIndex or stretchLevel changes, refresh the sliding window.
  $effect(() => {
    void currentIndex;
    void stretchLevel;
    updateWindow();
  });

  function updateWindow() {
    const n = frames.length;
    if (n === 0) return;

    // Compute the set of indices we want loaded.
    const wanted = new Set<number>();
    for (let d = -BEHIND; d <= AHEAD; d++) {
      wanted.add(((currentIndex + d) % n + n) % n);
    }

    // Evict entries outside the window.
    for (const [path] of cache) {
      const idx = frames.findIndex((f) => f.nasPath === path);
      if (idx === -1 || !wanted.has(idx)) {
        cache.delete(path);
      }
    }

    // Fetch missing entries.
    for (const idx of wanted) {
      const path = frames[idx].nasPath;
      if (!cache.has(path) && !fetching.has(path)) {
        fetching.add(path);
        GeneratePreview(path, stretchLevel)
          .then((url) => {
            cache.set(path, url);
            fetching.delete(path);
            cacheVersion++;
          })
          .catch(() => {
            cache.set(path, ""); // empty = failed, don't retry
            fetching.delete(path);
            cacheVersion++;
          });
      }
    }
  }

  function startBlink() {
    if (timerId) return;
    timerId = setInterval(() => {
      currentIndex = (currentIndex + 1) % frames.length;
    }, intervalMs);
    playing = true;
  }

  function stopBlink() {
    if (timerId) {
      clearInterval(timerId);
      timerId = null;
    }
    playing = false;
  }

  function togglePlay() {
    if (playing) stopBlink();
    else startBlink();
  }

  function step(dir: -1 | 1) {
    stopBlink();
    currentIndex = (currentIndex + dir + frames.length) % frames.length;
  }

  function onSpeedChange(e: Event) {
    intervalMs = parseInt((e.target as HTMLInputElement).value);
    if (playing) {
      stopBlink();
      startBlink();
    }
  }

  function advanceAfterAction() {
    if (frames.length <= 1) {
      onclose();
    } else if (currentIndex < frames.length - 1) {
      currentIndex++;
    } else {
      currentIndex = 0;
    }
  }

  function doReject() {
    const frame = frames[currentIndex];
    if (!frame || !onreject) return;
    stopBlink();
    onreject(frame.nasPath);
    advanceAfterAction();
  }

  function doHardDelete() {
    const frame = frames[currentIndex];
    if (!frame || !onharddelete) return;
    stopBlink();
    confirmDelete = false;
    onharddelete(frame.nasPath, frame.fileName);
    advanceAfterAction();
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      if (confirmDelete) confirmDelete = false;
      else onclose();
    }
    if (e.key === " ") {
      e.preventDefault();
      togglePlay();
    }
    if (e.key === "ArrowRight") step(1);
    if (e.key === "ArrowLeft") step(-1);
  }

  onDestroy(() => stopBlink());

  const current = $derived(frames[currentIndex]);
</script>

<div
  class="blink-backdrop"
  onmousedown={(e) => {
    if (e.target === e.currentTarget) onclose();
  }}
  onkeydown={(e) => e.key === "Escape" && onclose()}
  role="presentation"
>
  <div class="blink-modal" role="dialog" tabindex="-1" aria-modal="true" onkeydown={onKeydown}>
    <div class="blink-header">
      <span class="blink-title">Blink Comparison — {frames.length} frames</span>
      <button class="blink-close" onclick={onclose}>✕</button>
    </div>

    <div class="blink-viewport">
      {#if currentLoading}
        <div class="blink-loading">Loading…</div>
      {:else if currentPreview}
        <img
          src={currentPreview}
          alt={current?.fileName ?? ""}
          class="blink-img"
          draggable="false"
        />
      {:else}
        <div class="blink-loading">Preview unavailable</div>
      {/if}
      <div class="blink-badge">{currentIndex + 1} / {frames.length}</div>
      {#if current?.isRejected}
        <div class="blink-rejected-badge">REJECTED</div>
      {/if}
      <!-- Preload indicator: how many of the window are ready -->
      {#if frames.length > WINDOW}
        {@const ready = [currentIndex, ...Array.from({length: AHEAD}, (_, i) => ((currentIndex + i + 1) % frames.length))].filter(i => cache.has(frames[i]?.nasPath ?? "")).length}
        {@const total = Math.min(WINDOW, frames.length)}
        {#if ready < total}
          <div class="blink-preload-badge">⟳ {ready}/{total}</div>
        {/if}
      {/if}
    </div>

    <div class="blink-info">
      <span class="blink-name" title={current?.nasPath}>{current?.fileName}</span>
      {#if current?.dateObs}
        <span class="blink-date">{current.dateObs.slice(0, 16).replace("T", " ")}</span>
      {/if}
      {#if current?.fwhm && current.qualityAnalyzed}
        <span class="blink-stat">FWHM {current.fwhm.toFixed(2)}{current.fwhmUnit || "px"}</span>
      {/if}
      {#if current?.object}
        <span class="blink-obj">{current.object}</span>
      {/if}
    </div>

    <div class="blink-controls">
      <button class="blink-btn" onclick={() => step(-1)} title="Previous (←)">◀</button>
      <button
        class="blink-btn blink-play"
        class:playing
        onclick={togglePlay}
        title="Play/Pause (Space)"
      >
        {playing ? "⏸" : "▶"}
      </button>
      <button class="blink-btn" onclick={() => step(1)} title="Next (→)">▶</button>

      {#if onreject || onharddelete}
        <div class="blink-actions">
          {#if onreject}
            <button
              class="blink-action-btn reject-btn"
              disabled={current?.isRejected}
              onclick={doReject}
              title={current?.isRejected ? "Already rejected" : "Reject this frame"}
            >
              ✕ Reject
            </button>
          {/if}
          {#if onharddelete}
            {#if confirmDelete}
              <span class="blink-confirm-text">Delete permanently?</span>
              <button class="blink-action-btn delete-confirm-btn" onclick={doHardDelete}
                >Yes, delete</button
              >
              <button class="blink-action-btn cancel-btn" onclick={() => (confirmDelete = false)}
                >Cancel</button
              >
            {:else}
              <button
                class="blink-action-btn delete-btn"
                onclick={() => (confirmDelete = true)}
                title="Hard delete this frame from disk"
              >
                🗑 Delete
              </button>
            {/if}
          {/if}
        </div>
      {/if}

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
        {#each [-2, -1, 0, 1, 2] as offset (offset)}
          {@const idx = ((currentIndex + offset) % frames.length + frames.length) % frames.length}
          {@const frame = frames[idx]}
          {#if frame}
            <button
              class="blink-dot"
              class:active={offset === 0}
              class:rejected={frame.isRejected}
              class:cached={cache.has(frame.nasPath)}
              onclick={() => { stopBlink(); currentIndex = idx; }}
              title={frame.fileName}
            ></button>
          {/if}
        {/each}
      </div>

    </div>
  </div>
</div>

<style>
  .blink-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.72);
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
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.7);
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

  .blink-title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text-primary);
  }
  .blink-close {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    font-size: 1rem;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
  }
  .blink-close:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }

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
    background: rgba(0, 0, 0, 0.6);
    color: var(--text-secondary);
    font-size: 0.72rem;
    padding: 2px 7px;
    border-radius: 3px;
    font-variant-numeric: tabular-nums;
  }

  .blink-preload-badge {
    position: absolute;
    bottom: 8px;
    left: 8px;
    background: rgba(0, 0, 0, 0.5);
    color: var(--text-secondary);
    font-size: 0.68rem;
    padding: 2px 6px;
    border-radius: 3px;
  }

  .blink-rejected-badge {
    position: absolute;
    top: 8px;
    right: 8px;
    background: rgba(180, 40, 40, 0.8);
    color: #fff;
    font-size: 0.68rem;
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 3px;
    letter-spacing: 0.06em;
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

  .blink-name {
    font-size: 0.8rem;
    font-family: "Consolas", monospace;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }
  .blink-date {
    font-size: 0.75rem;
    color: var(--text-secondary);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
  .blink-stat {
    font-size: 0.75rem;
    color: var(--accent);
    white-space: nowrap;
  }
  .blink-obj {
    font-size: 0.75rem;
    color: var(--text-secondary);
    white-space: nowrap;
  }

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
    transition:
      background 0.1s,
      color 0.1s;
    min-width: 32px;
  }
  .blink-btn:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .blink-play {
    min-width: 44px;
    font-size: 1rem;
  }
  .blink-play.playing {
    color: var(--accent);
    border-color: var(--accent);
  }

  .blink-speed {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: 8px;
  }
  .blink-speed-label {
    font-size: 0.72rem;
    color: var(--text-secondary);
  }
  .blink-slider {
    width: 90px;
    accent-color: var(--accent);
    cursor: pointer;
  }
  .blink-speed-val {
    font-size: 0.72rem;
    color: var(--text-secondary);
    width: 42px;
    font-variant-numeric: tabular-nums;
  }

  .blink-dots {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;
    flex-wrap: wrap;
    max-width: 300px;
  }
  .blink-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--border-accent);
    border: none;
    cursor: pointer;
    padding: 0;
    transition:
      background 0.1s,
      transform 0.1s;
  }
  .blink-dot.active {
    background: var(--accent);
    transform: scale(1.4);
  }
  .blink-dot.rejected {
    background: var(--danger);
  }
  .blink-dot.cached {
    opacity: 1;
  }
  .blink-dot:not(.cached):not(.active) {
    opacity: 0.4;
  }
  .blink-dot:hover {
    background: var(--text-secondary);
  }

  /* ── Frame actions ────────────────────────────────────────────────────────── */
  .blink-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: 8px;
    flex-shrink: 0;
  }

  .blink-action-btn {
    font-size: 0.75rem;
    padding: 3px 10px;
    border-radius: 4px;
    cursor: pointer;
    border: 1px solid;
    transition:
      background 0.1s,
      color 0.1s;
    white-space: nowrap;
  }

  .reject-btn {
    background: transparent;
    border-color: var(--danger);
    color: var(--danger);
  }
  .reject-btn:hover:not(:disabled) {
    background: var(--danger);
    color: #fff;
  }
  .reject-btn:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }

  .delete-btn {
    background: transparent;
    border-color: var(--border);
    color: var(--text-secondary);
  }
  .delete-btn:hover {
    border-color: var(--danger);
    color: var(--danger);
  }

  .delete-confirm-btn {
    background: var(--danger);
    border-color: var(--danger);
    color: #fff;
    font-weight: 600;
  }
  .delete-confirm-btn:hover {
    opacity: 0.85;
  }

  .cancel-btn {
    background: transparent;
    border-color: var(--border);
    color: var(--text-secondary);
  }
  .cancel-btn:hover {
    background: var(--bg-row-hover);
  }

  .blink-confirm-text {
    font-size: 0.75rem;
    color: var(--danger);
    white-space: nowrap;
  }
</style>
