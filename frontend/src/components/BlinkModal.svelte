<script lang="ts">
  import { onDestroy, onMount, untrack } from "svelte";
  import type * as app from "$models/app";
  import { GeneratePreview } from "$app";
  import { pushModal, popModal, isTopModal } from "../lib/modalStack";
  import { BlinkLoader, preloadOrder } from "../lib/library/blinkLoader";

  interface Props {
    frames: app.LibraryFrame[];
    onclose: () => void;
    onreject?: (nasPath: string) => void;
    onrestore?: (nasPath: string) => void;
    /**
     * Deletes the frame from disk. Blink asks for confirmation itself, so the handler
     * must not confirm again; the parent removes the frame from `frames` on success.
     */
    onharddelete?: (nasPath: string, name: string) => void;
    /** Frame to start on (defaults to the first). */
    startPath?: string | null;
  }

  let { frames, onclose, onreject, onrestore, onharddelete, startPath = null }: Props = $props();

  // ── State ─────────────────────────────────────────────────────────────────
  // svelte-ignore state_referenced_locally
  let currentIndex = $state(
    Math.max(
      0,
      frames.findIndex((f) => f.nasPath === startPath),
    ),
  );
  let intervalMs = $state(500);
  let playing = $state(false);
  let stretchLevel = $state(2);
  let confirmDelete = $state(false);

  // Frames ahead/behind the current one to preload (nearest first).
  const AHEAD = 8;
  const BEHIND = 3;

  let timerId: ReturnType<typeof setInterval> | null = null;

  // The loader isn't reactive; bump a version so the derived values re-read it.
  let loadVersion = $state(0);
  const loader = new BlinkLoader(
    (path, level) => GeneratePreview(path, level),
    () => loadVersion++,
  );

  const currentPreview = $derived.by(() => {
    void loadVersion;
    const path = frames[currentIndex]?.nasPath;
    return path ? (loader.get(path, stretchLevel) ?? null) : null;
  });
  const currentLoading = $derived.by(() => {
    void loadVersion;
    const path = frames[currentIndex]?.nasPath;
    return !!path && !loader.settled(path, stretchLevel);
  });
  /** Reactive wrapper: re-evaluates in templates whenever a frame loads. */
  function isSettled(path: string): boolean {
    void loadVersion;
    return loader.settled(path, stretchLevel);
  }

  /** How many of the next AHEAD frames are ready (for the preload badge). */
  const readyAhead = $derived.by(() => {
    void loadVersion;
    const n = frames.length;
    let ready = 0;
    for (let k = 1; k <= Math.min(AHEAD, n - 1); k++) {
      if (loader.settled(frames[(currentIndex + k) % n].nasPath, stretchLevel)) ready++;
    }
    return ready;
  });

  // Keep pointing at the same frame when the parent updates `frames` (reject flag
  // flips, deletions). If the current frame was removed, the next one slides into
  // its index; past the end we wrap to the first frame.
  let lastPath = "";
  $effect(() => {
    const fs = frames;
    untrack(() => {
      const i = fs.findIndex((f) => f.nasPath === lastPath);
      if (i !== -1) currentIndex = i;
      else if (currentIndex >= fs.length) currentIndex = 0;
      if (fs.length < 2) stopBlink();
    });
  });
  $effect(() => {
    lastPath = frames[currentIndex]?.nasPath ?? "";
  });

  // Whenever the frame set, currentIndex or stretchLevel changes, refresh the window.
  $effect(() => {
    void frames;
    void currentIndex;
    void stretchLevel;
    untrack(updateWindow);
  });

  function updateWindow() {
    const n = frames.length;
    if (n === 0) return;
    const order = preloadOrder(currentIndex, n, AHEAD, BEHIND);
    loader.want(
      order.map((i) => frames[i].nasPath),
      stretchLevel,
    );
  }

  function startBlink() {
    if (timerId || frames.length < 2) return;
    timerId = setInterval(() => {
      // Hold on the current frame until the next one is ready, rather than
      // flashing "Loading…" mid-blink.
      const next = (currentIndex + 1) % frames.length;
      if (loader.settled(frames[next].nasPath, stretchLevel)) currentIndex = next;
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
    confirmDelete = false;
    if (frames.length === 0) return;
    currentIndex = (currentIndex + dir + frames.length) % frames.length;
  }

  function onSpeedChange(e: Event) {
    intervalMs = parseInt((e.target as HTMLInputElement).value);
    if (playing) {
      stopBlink();
      startBlink();
    }
  }

  function advance() {
    if (frames.length > 1) currentIndex = (currentIndex + 1) % frames.length;
  }

  function doReject() {
    const frame = frames[currentIndex];
    if (!frame || !onreject || frame.isRejected) return;
    stopBlink();
    onreject(frame.nasPath);
    advance();
  }

  function doRestore() {
    const frame = frames[currentIndex];
    if (!frame || !onrestore || !frame.isRejected) return;
    stopBlink();
    onrestore(frame.nasPath);
  }

  function askDelete() {
    if (!onharddelete || !frames[currentIndex]) return;
    stopBlink();
    confirmDelete = true;
  }

  function doHardDelete() {
    const frame = frames[currentIndex];
    confirmDelete = false;
    if (!frame || !onharddelete) return;
    stopBlink();
    // The parent removes the frame from `frames`; the effect above keeps the index valid.
    onharddelete(frame.nasPath, frame.fileName);
  }

  // ── Keyboard / modal stack ────────────────────────────────────────────────
  let dialogEl: HTMLDivElement;
  let modalId: symbol | null = null;
  const previouslyFocused =
    document.activeElement instanceof HTMLElement ? document.activeElement : null;

  onMount(() => {
    modalId = pushModal();
    dialogEl.focus();
    return () => {
      if (modalId) popModal(modalId);
      if (previouslyFocused?.isConnected) previouslyFocused.focus();
    };
  });

  function onWindowKeydown(e: KeyboardEvent) {
    if (!modalId || !isTopModal(modalId)) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const inInput = e.target instanceof HTMLInputElement;

    switch (e.key) {
      case "Escape":
        e.preventDefault();
        if (confirmDelete) confirmDelete = false;
        else onclose();
        return;
      case " ":
        e.preventDefault();
        togglePlay();
        return;
      case "ArrowRight":
      case "ArrowLeft":
        // Let the speed slider keep its own arrow-key behaviour.
        if (inInput) return;
        e.preventDefault();
        step(e.key === "ArrowRight" ? 1 : -1);
        return;
      case "r":
        e.preventDefault();
        doReject();
        return;
      case "u":
        e.preventDefault();
        doRestore();
        return;
      case "Delete":
        e.preventDefault();
        if (confirmDelete) doHardDelete();
        else askDelete();
        return;
      case "Enter":
        if (confirmDelete) {
          e.preventDefault();
          doHardDelete();
        }
        return;
    }
  }

  onDestroy(() => stopBlink());

  const current = $derived(frames[currentIndex]);
</script>

<svelte:window onkeydown={onWindowKeydown} />

<div
  class="blink-backdrop"
  onmousedown={(e) => {
    if (e.target === e.currentTarget) onclose();
  }}
  role="presentation"
>
  <div
    bind:this={dialogEl}
    class="blink-modal"
    role="dialog"
    tabindex="-1"
    aria-modal="true"
    aria-label="Blink comparison"
  >
    <div class="blink-header">
      <span class="blink-title">Blink comparison — {frames.length} frames</span>
      <button class="blink-close" onclick={onclose} aria-label="Close blink" title="Close (Esc)"
        >✕</button
      >
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
      {#if readyAhead < Math.min(AHEAD, frames.length - 1)}
        <div class="blink-preload-badge" title="Frames ahead that are ready">
          ⟳ {readyAhead}/{Math.min(AHEAD, frames.length - 1)}
        </div>
      {/if}
    </div>

    <div class="blink-info">
      <span class="blink-name" title={current?.nasPath}>{current?.fileName}</span>
      {#if current?.dateObs}
        <span class="blink-meta">{current.dateObs.slice(0, 16).replace("T", " ")}</span>
      {/if}
      {#if current?.fwhm && current.qualityAnalyzed}
        <span class="blink-stat">FWHM {current.fwhm.toFixed(2)}{current.fwhmUnit || "px"}</span>
      {/if}
      {#if current?.object}
        <span class="blink-meta">{current.object}</span>
      {/if}
    </div>

    <div class="blink-controls">
      <button class="blink-btn" onclick={() => step(-1)} title="Previous (←)" aria-label="Previous"
        >◀</button
      >
      <button
        class="blink-btn blink-play"
        class:playing
        onclick={togglePlay}
        title="Play/Pause (Space)"
        aria-label={playing ? "Pause" : "Play"}
      >
        {playing ? "⏸" : "▶"}
      </button>
      <button class="blink-btn" onclick={() => step(1)} title="Next (→)" aria-label="Next">▶</button
      >

      {#if onreject || onrestore || onharddelete}
        <div class="blink-actions">
          {#if current?.isRejected && onrestore}
            <button class="blink-action-btn restore-btn" onclick={doRestore} title="Restore (u)">
              Restore
            </button>
          {:else if onreject}
            <button
              class="blink-action-btn reject-btn"
              disabled={current?.isRejected}
              onclick={doReject}
              title={current?.isRejected ? "Already rejected" : "Reject this frame (r)"}
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
                onclick={askDelete}
                title="Delete this frame from disk (Delete)"
              >
                Delete…
              </button>
            {/if}
          {/if}
        </div>
      {/if}

      <label class="blink-speed">
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
      </label>

      <div class="blink-dots">
        {#each [-2, -1, 0, 1, 2] as offset (offset)}
          {@const idx = (((currentIndex + offset) % frames.length) + frames.length) % frames.length}
          {@const frame = frames[idx]}
          {#if frame}
            <button
              class="blink-dot"
              class:active={offset === 0}
              class:rejected={frame.isRejected}
              class:pending={!isSettled(frame.nasPath)}
              onclick={() => {
                stopBlink();
                currentIndex = idx;
              }}
              title={frame.fileName}
              aria-label="Show {frame.fileName}"
            ></button>
          {/if}
        {/each}
      </div>
    </div>

    <div class="blink-legend" aria-label="Keyboard shortcuts">
      <span><kbd>←</kbd><kbd>→</kbd> step</span>
      <span><kbd>Space</kbd> play/pause</span>
      {#if onreject}<span><kbd>r</kbd> reject</span>{/if}
      {#if onrestore}<span><kbd>u</kbd> restore</span>{/if}
      {#if onharddelete}<span><kbd>Del</kbd> delete</span>{/if}
      <span><kbd>Esc</kbd> close</span>
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
    height: min(88vh, 820px);
    box-shadow: 0 16px 48px rgba(0, 0, 0, 0.7);
    overflow: hidden;
    outline: none;
  }

  .blink-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px 8px 14px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .blink-title {
    font-size: var(--fs-md);
    font-weight: 600;
    color: var(--text-primary);
  }
  .blink-close {
    width: 28px;
    height: 28px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: var(--fs-md);
    cursor: pointer;
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
    font-size: var(--fs-md);
  }

  .blink-badge {
    position: absolute;
    top: 8px;
    left: 8px;
    background: rgba(0, 0, 0, 0.6);
    color: var(--text-primary);
    font-size: var(--fs-xs);
    padding: 2px 7px;
    border-radius: 3px;
    font-variant-numeric: tabular-nums;
  }

  .blink-preload-badge {
    position: absolute;
    bottom: 8px;
    left: 8px;
    background: rgba(0, 0, 0, 0.6);
    color: var(--text-secondary);
    font-size: var(--fs-xs);
    padding: 2px 6px;
    border-radius: 3px;
  }

  .blink-rejected-badge {
    position: absolute;
    top: 8px;
    right: 8px;
    background: var(--danger);
    color: #fff;
    font-size: var(--fs-xs);
    font-weight: 700;
    padding: 2px 8px;
    border-radius: 3px;
    letter-spacing: 0.06em;
  }

  .blink-info {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 6px 14px;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .blink-name {
    font-size: var(--fs-sm);
    font-family: "Consolas", monospace;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }
  .blink-meta {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
  .blink-stat {
    font-size: var(--fs-xs);
    color: var(--accent);
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
    font-size: var(--fs-sm);
    padding: 4px 10px;
    cursor: pointer;
    transition:
      background 0.1s,
      color 0.1s;
    min-width: 32px;
    min-height: 28px;
  }
  .blink-btn:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }
  .blink-play {
    min-width: 44px;
    font-size: var(--fs-lg);
  }
  .blink-play.playing {
    background: var(--accent);
    color: var(--accent-contrast);
    border-color: var(--accent);
  }

  .blink-speed {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: 8px;
  }
  .blink-speed-label {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .blink-slider {
    width: 90px;
    accent-color: var(--accent);
    cursor: pointer;
  }
  .blink-speed-val {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    width: 48px;
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
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--border-accent);
    border: none;
    cursor: pointer;
    padding: 0;
    transition:
      background 0.1s,
      transform 0.1s;
  }
  .blink-dot.pending {
    background: var(--border);
  }
  .blink-dot.active {
    background: var(--accent);
    transform: scale(1.3);
  }
  .blink-dot.rejected {
    background: var(--danger);
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
    font-size: var(--fs-xs);
    padding: 4px 10px;
    min-height: 28px;
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
    border-color: var(--border);
    color: var(--text-dim);
    cursor: not-allowed;
  }

  .restore-btn {
    background: transparent;
    border-color: var(--accent);
    color: var(--accent);
  }
  .restore-btn:hover {
    background: var(--accent);
    color: var(--accent-contrast);
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

  .cancel-btn {
    background: transparent;
    border-color: var(--border);
    color: var(--text-secondary);
  }
  .cancel-btn:hover {
    background: var(--bg-row-hover);
  }

  .blink-confirm-text {
    font-size: var(--fs-xs);
    color: var(--danger);
    white-space: nowrap;
  }

  /* ── Shortcut legend ──────────────────────────────────────────────────────── */
  .blink-legend {
    display: flex;
    flex-wrap: wrap;
    gap: 4px 14px;
    padding: 6px 14px 8px;
    border-top: 1px solid var(--border);
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    flex-shrink: 0;
  }
  .blink-legend kbd {
    display: inline-block;
    min-width: 18px;
    padding: 0 4px;
    margin-right: 3px;
    border: 1px solid var(--border-accent);
    border-radius: 3px;
    background: var(--bg-base);
    color: var(--text-primary);
    font-family: inherit;
    font-size: var(--fs-xs);
    text-align: center;
  }
</style>
