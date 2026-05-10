<script lang="ts">
  import { untrack } from 'svelte'
  import type { browser, fits } from '../../wailsjs/go/models'
  import { GeneratePreview, ReadFITSHeader } from '../../wailsjs/go/app/App.js'
  import { basicRows, advancedRows } from '../lib/utils'

  interface Props {
    entry:             browser.EnrichedFileEntry
    stretchEnabled:    boolean
    stretchLevel:      number
    basicCollapsed:    boolean
    advancedCollapsed: boolean
    onclose:           () => void
  }

  let {
    entry,
    stretchEnabled    = $bindable(),
    stretchLevel      = $bindable(),
    basicCollapsed    = $bindable(),
    advancedCollapsed = $bindable(),
    onclose,
  }: Props = $props()

  // ── Preview load state ───────────────────────────────────────────────────
  let previewDataUrl = $state('')
  let previewLoading = $state(false)
  let previewError   = $state('')
  let fitsHeader     = $state<fits.FITSHeader | null>(null)
  let previewReqId   = 0

  // ── Zoom / pan ────────────────────────────────────────────────────────────
  let zoom      = $state(1)
  let panX      = $state(0)
  let panY      = $state(0)
  let isPanning = $state(false)
  let panStartX = 0
  let panStartY = 0

  function resetView() { zoom = 1; panX = 0; panY = 0 }

  // When entry changes, reload everything.
  $effect(() => {
    const e = entry
    previewDataUrl = ''
    previewError   = ''
    fitsHeader     = null
    previewLoading = true
    resetView()

    const id    = ++previewReqId
    const level = untrack(() => stretchEnabled ? stretchLevel : 0)

    Promise.allSettled([
      ReadFITSHeader(e.path),
      GeneratePreview(e.path, level),
    ]).then(([hdrResult, imgResult]) => {
      if (id !== previewReqId) return
      previewLoading = false
      if (hdrResult.status === 'fulfilled') fitsHeader = hdrResult.value
      if (imgResult.status === 'fulfilled') {
        previewDataUrl = imgResult.value
      } else {
        previewError = (imgResult as PromiseRejectedResult).reason?.toString() ?? 'Preview failed'
      }
    })
  })

  async function refreshPreview() {
    const id    = ++previewReqId
    previewLoading = true
    previewError   = ''
    const level = stretchEnabled ? stretchLevel : 0
    try {
      const result = await GeneratePreview(entry.path, level)
      if (id !== previewReqId) return
      previewDataUrl = result
    } catch (e) {
      if (id !== previewReqId) return
      previewError = String(e) || 'Preview failed'
    } finally {
      if (id === previewReqId) previewLoading = false
    }
  }

  function setStretch(level: number) {
    stretchLevel = level
    refreshPreview()
  }

  // ── Wheel zoom ────────────────────────────────────────────────────────────
  function onWheel(e: WheelEvent) {
    e.preventDefault()
    const factor  = e.deltaY < 0 ? 1.15 : 0.87
    const newZoom = Math.max(0.25, Math.min(20, zoom * factor))
    const rect    = (e.currentTarget as HTMLElement).getBoundingClientRect()
    const mx      = e.clientX - rect.left - rect.width / 2
    const my      = e.clientY - rect.top  - rect.height / 2
    panX = mx - (mx - panX) * newZoom / zoom
    panY = my - (my - panY) * newZoom / zoom
    zoom = newZoom
  }

  function onPanStart(e: MouseEvent) {
    if (e.button !== 0) return
    isPanning = true
    panStartX = e.clientX - panX
    panStartY = e.clientY - panY
  }
  function onPanMove(e: MouseEvent) {
    if (!isPanning) return
    panX = e.clientX - panStartX
    panY = e.clientY - panStartY
  }
  function onPanEnd() { isPanning = false }
</script>

<div class="preview-pane">

  <div class="preview-titlebar">
    <span class="preview-filename" title={entry.path}>{entry.name}</span>
    <div class="preview-controls">
      <span class="zoom-label">{Math.round(zoom * 100)}%</span>
      <button class="tool-btn" onclick={resetView} title="Fit to window (or double-click image)">Fit</button>
      <div class="stretch-group">
        <button
          class="tool-btn"
          class:active={stretchEnabled}
          onclick={() => { stretchEnabled = !stretchEnabled; refreshPreview() }}
          title="Toggle autostretch"
        >Stretch</button>
        {#if stretchEnabled}
          <button class="tool-btn preset" class:active={stretchLevel === 1} onclick={() => setStretch(1)}>Gentle</button>
          <button class="tool-btn preset" class:active={stretchLevel === 2} onclick={() => setStretch(2)}>Normal</button>
          <button class="tool-btn preset" class:active={stretchLevel === 3} onclick={() => setStretch(3)}>Strong</button>
        {/if}
      </div>
      <button class="btn-icon small" onclick={onclose} title="Close preview">✕</button>
    </div>
  </div>

  <div
    class="image-viewport"
    class:panning={isPanning}
    onwheel={onWheel}
    onmousedown={onPanStart}
    onmousemove={onPanMove}
    onmouseup={onPanEnd}
    onmouseleave={onPanEnd}
    ondblclick={resetView}
  >
    {#if previewLoading}
      <div class="preview-status">
        <span class="spinner">◌</span> Generating preview…
      </div>
    {:else if previewError}
      <div class="preview-error">{previewError}</div>
    {:else if previewDataUrl}
      <img
        class="preview-img"
        src={previewDataUrl}
        alt={entry.name}
        style="transform: translate({panX}px, {panY}px) scale({zoom});"
        draggable="false"
      />
    {/if}
  </div>

  {#if fitsHeader}
    <div class="preview-meta">
      <div class="meta-section">
        <button class="meta-section-hdr" onclick={() => (basicCollapsed = !basicCollapsed)}>
          <span>Basic</span>
          <span class="meta-caret">{basicCollapsed ? '›' : '⌄'}</span>
        </button>
        {#if !basicCollapsed}
          {#each basicRows(fitsHeader) as row}
            <div class="meta-row">
              <span class="meta-key">{row.key}</span>
              <span class="meta-val">{row.val}</span>
            </div>
          {/each}
        {/if}
      </div>
      <div class="meta-section">
        <button class="meta-section-hdr" onclick={() => (advancedCollapsed = !advancedCollapsed)}>
          <span>Advanced</span>
          <span class="meta-caret">{advancedCollapsed ? '›' : '⌄'}</span>
        </button>
        {#if !advancedCollapsed}
          {#each advancedRows(fitsHeader) as row}
            <div class="meta-row">
              <span class="meta-key">{row.key}</span>
              <span class="meta-val">{row.val}</span>
            </div>
          {/each}
        {/if}
      </div>
    </div>
  {/if}

</div>

<style>
  .preview-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
  }

  .preview-titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 6px 12px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .preview-filename {
    font-size: 0.8rem;
    font-family: 'Consolas', 'Fira Code', monospace;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }

  .preview-controls {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }

  .zoom-label {
    font-size: 0.75rem;
    color: var(--text-dim);
    font-variant-numeric: tabular-nums;
    min-width: 36px;
    text-align: right;
  }

  .stretch-group { display: flex; align-items: center; gap: 3px; }

  /* ── Image viewport ───────────────────────────────────────────────────── */

  .image-viewport {
    flex: 1;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: grab;
    position: relative;
    background: #08090f;
  }

  .image-viewport.panning { cursor: grabbing; }

  .preview-img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    user-select: none;
    pointer-events: none;
    transform-origin: center;
    will-change: transform;
    display: block;
  }

  .preview-status {
    color: var(--text-dim);
    font-size: 0.875rem;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .spinner {
    display: inline-block;
    animation: spin 1.2s linear infinite;
    color: var(--accent);
    font-size: 1.2rem;
  }

  @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

  .preview-error {
    color: var(--danger);
    font-size: 0.82rem;
    background: #2a1020;
    border: 1px solid var(--danger);
    border-radius: 4px;
    padding: 10px 14px;
    max-width: 80%;
  }

  /* ── FITS metadata ────────────────────────────────────────────────────── */

  .preview-meta {
    flex-shrink: 0;
    padding: 0 14px 10px;
    overflow-y: auto;
    max-height: 220px;
    border-top: 1px solid var(--border);
  }

  .preview-meta::-webkit-scrollbar { width: 4px; }
  .preview-meta::-webkit-scrollbar-thumb { background: var(--border-accent); border-radius: 2px; }

  .meta-section { border-bottom: 1px solid var(--border); }

  .meta-section-hdr {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 5px 0 3px;
    color: var(--text-dim);
    font-size: 0.68rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .meta-section-hdr:hover { color: var(--accent); }
  .meta-caret { font-size: 0.8rem; opacity: 0.6; }

  .meta-row {
    display: flex;
    justify-content: space-between;
    padding: 2px 0;
    font-size: 0.79rem;
    border-bottom: 1px solid var(--border);
  }

  .meta-key { color: var(--text-dim); width: 80px; flex-shrink: 0; }
  .meta-val {
    color: var(--text-primary);
    text-align: right;
    font-variant-numeric: tabular-nums;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
