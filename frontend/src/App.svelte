<script>
  import { onMount } from 'svelte'
  import {
    SelectRootFolder,
    ListDirectory,
    GeneratePreview,
    ReadFITSHeader,
    LoadPrefs,
    SetPref,
  } from '../wailsjs/go/app/App.js'

  // Preference keys — mirrors internal/prefs/prefs.go constants.
  const PREF_ROOT_FOLDER        = 'root_folder'
  const PREF_BASIC_COLLAPSED    = 'basic_collapsed'
  const PREF_ADVANCED_COLLAPSED = 'advanced_collapsed'
  const PREF_STRETCH_ENABLED    = 'stretch_enabled'
  const PREF_STRETCH_LEVEL      = 'stretch_level'

  // ── File browser state ────────────────────────────────────────────────────
  let rootFolder = ''
  let currentPath = ''
  let pathHistory = []
  let files = []
  let error = ''
  let loading = false

  // ── Preview state ─────────────────────────────────────────────────────────
  let selectedEntry = null
  let previewDataUrl = ''
  let previewLoading = false
  let previewError = ''
  let fitsHeader = null

  // ── Stretch controls ──────────────────────────────────────────────────────
  let stretchEnabled = true
  let stretchLevel = 2   // 1=gentle 2=normal 3=strong

  // ── Header section collapse ───────────────────────────────────────────────
  let basicCollapsed = false
  let advancedCollapsed = true

  // ── Preview request ID (stale cancellation) ───────────────────────────────
  let previewReqId = 0

  // ── Preference persistence ────────────────────────────────────────────────
  // Guard: reactive saves only fire AFTER the initial load from the DB.
  let prefsLoaded = false

  onMount(async () => {
    const p = await LoadPrefs()
    // Seed all preference-backed state from the DB before marking as loaded.
    stretchEnabled    = p.stretchEnabled
    stretchLevel      = p.stretchLevel
    basicCollapsed    = p.basicCollapsed
    advancedCollapsed = p.advancedCollapsed
    if (p.rootFolder) {
      rootFolder = p.rootFolder
      await loadDirectory(p.rootFolder)
    }
    prefsLoaded = true
  })

  // Auto-save each preference when it changes (guarded by prefsLoaded).
  $: if (prefsLoaded) SetPref(PREF_STRETCH_ENABLED,    String(stretchEnabled))
  $: if (prefsLoaded) SetPref(PREF_STRETCH_LEVEL,      String(stretchLevel))
  $: if (prefsLoaded) SetPref(PREF_BASIC_COLLAPSED,    String(basicCollapsed))
  $: if (prefsLoaded) SetPref(PREF_ADVANCED_COLLAPSED, String(advancedCollapsed))

  // ── Pane resize ───────────────────────────────────────────────────────────
  let leftPct = 40       // left pane width as % of content area
  let collapsed = false  // left pane collapsed

  function onDividerMouseDown(e) {
    e.preventDefault()
    const contentArea = document.querySelector('.content-area')

    function onMove(e) {
      const rect = contentArea.getBoundingClientRect()
      const pct = ((e.clientX - rect.left) / rect.width) * 100
      leftPct = Math.max(15, Math.min(75, pct))
      if (collapsed) collapsed = false
    }
    function onUp() {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  // ── Zoom / pan ────────────────────────────────────────────────────────────
  let zoom = 1
  let panX = 0
  let panY = 0
  let isPanning = false
  let panStartX = 0
  let panStartY = 0

  function resetView() { zoom = 1; panX = 0; panY = 0 }

  function onWheel(e) {
    e.preventDefault()
    const factor = e.deltaY < 0 ? 1.15 : 0.87
    const newZoom = Math.max(0.25, Math.min(20, zoom * factor))
    const rect = e.currentTarget.getBoundingClientRect()
    const mx = e.clientX - rect.left - rect.width / 2
    const my = e.clientY - rect.top - rect.height / 2
    panX = mx - (mx - panX) * newZoom / zoom
    panY = my - (my - panY) * newZoom / zoom
    zoom = newZoom
  }

  function onPanStart(e) {
    if (e.button !== 0) return
    isPanning = true
    panStartX = e.clientX - panX
    panStartY = e.clientY - panY
  }
  function onPanMove(e) {
    if (!isPanning) return
    panX = e.clientX - panStartX
    panY = e.clientY - panStartY
  }
  function onPanEnd() { isPanning = false }

  // ── Navigation ────────────────────────────────────────────────────────────
  function isFits(name) {
    const l = name.toLowerCase()
    return l.endsWith('.fits') || l.endsWith('.fit')
  }

  async function selectFolder() {
    const path = await SelectRootFolder()
    if (path) {
      rootFolder = path
      if (prefsLoaded) SetPref(PREF_ROOT_FOLDER, path)
      pathHistory = []
      clearPreview()
      await loadDirectory(path)
    }
  }

  async function loadDirectory(path) {
    loading = true
    error = ''
    try {
      const result = await ListDirectory(path)
      files = (result || []).slice().sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
        return a.name.localeCompare(b.name)
      })
      currentPath = path
    } catch (e) {
      error = e.toString()
      files = []
    } finally {
      loading = false
    }
  }

  async function onRowClick(entry) {
    if (entry.isDir) {
      clearPreview()
      pathHistory = [...pathHistory, currentPath]
      await loadDirectory(entry.path)
    } else if (isFits(entry.name)) {
      await openPreview(entry)
    }
  }

  async function openPreview(entry) {
    selectedEntry = entry
    previewDataUrl = ''
    previewError = ''
    fitsHeader = null
    previewLoading = true
    resetView()

    const id = ++previewReqId
    const level = stretchEnabled ? stretchLevel : 0

    const [hdrResult, imgResult] = await Promise.allSettled([
      ReadFITSHeader(entry.path),
      GeneratePreview(entry.path, level),
    ])

    if (id !== previewReqId) return
    previewLoading = false
    if (hdrResult.status === 'fulfilled') fitsHeader = hdrResult.value
    if (imgResult.status === 'fulfilled') {
      previewDataUrl = imgResult.value
    } else {
      previewError = imgResult.reason?.toString() ?? 'Preview failed'
    }
  }

  async function navigateBack() {
    if (pathHistory.length === 0) return
    const prev = pathHistory[pathHistory.length - 1]
    pathHistory = pathHistory.slice(0, -1)
    clearPreview()
    await loadDirectory(prev)
  }

  function clearPreview() {
    selectedEntry = null
    previewDataUrl = ''
    previewError = ''
    fitsHeader = null
    resetView()
  }

  async function refreshPreview() {
    if (!selectedEntry) return
    const id = ++previewReqId
    previewLoading = true
    previewError = ''
    const level = stretchEnabled ? stretchLevel : 0
    try {
      const result = await GeneratePreview(selectedEntry.path, level)
      if (id !== previewReqId) return
      previewDataUrl = result
    } catch (e) {
      if (id !== previewReqId) return
      previewError = e?.toString() ?? 'Preview failed'
    } finally {
      if (id === previewReqId) previewLoading = false
    }
  }

  function setStretch(level) {
    stretchLevel = level
    refreshPreview()
  }

  // ── Formatting helpers ────────────────────────────────────────────────────
  function formatDate(dateStr) {
    const d = new Date(dateStr)
    return d.toLocaleDateString('en-US', {
      year: 'numeric', month: 'short', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
    })
  }

  function formatSize(bytes, isDir) {
    if (isDir) return '—'
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
    return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
  }

  function truncatePath(path, maxLen = 60) {
    if (path.length <= maxLen) return path
    const parts = path.split('/')
    if (parts.length <= 2) return '…' + path.slice(-(maxLen - 1))
    return '…/' + parts.slice(-2).join('/')
  }

  function basicRows(h) {
    if (!h) return []
    const expStr = !h.exptime ? '—'
      : h.exptime >= 60 ? (h.exptime / 60).toFixed(1) + ' min'
      : h.exptime + ' s'
    return [
      { key: 'Object',   val: h.object  || '—' },
      { key: 'Filter',   val: h.filter  || '—' },
      { key: 'Exposure', val: expStr },
      { key: 'Date',     val: h.dateObs || '—' },
      { key: 'Size',     val: h.width && h.height
          ? `${h.width} × ${h.height}${h.channels > 1 ? ` × ${h.channels}` : ''}`
          : '—' },
    ]
  }

  function advancedRows(h) {
    if (!h) return []
    return [
      { key: 'Gain',      val: h.gain       || '—' },
      { key: 'CCD Temp',  val: h.ccdTemp ? h.ccdTemp + ' °C' : '—' },
      { key: 'Telescope', val: h.telescope  || '—' },
      { key: 'Camera',    val: h.instrument || '—' },
      { key: 'Binning',   val: h.xbinning ? `${h.xbinning} × ${h.ybinning}` : '—' },
    ]
  }

  $: leftStyle = selectedEntry
    ? collapsed
      ? 'flex: 0 0 0px; min-width: 0; overflow: hidden;'
      : `flex: 0 0 ${leftPct}%;`
    : 'flex: 1;'
</script>

<div class="layout">
  <header>
    <span class="logo">✦ Eirin</span>
    <div class="header-right">
      <button class="btn-primary" on:click={selectFolder}>
        {rootFolder ? 'Change Root Folder' : 'Select Root Folder'}
      </button>
    </div>
  </header>

  {#if !rootFolder}
    <div class="empty-state">
      <div class="empty-icon">◎</div>
      <p class="empty-title">No folder selected</p>
      <p class="empty-sub">Choose your astrophotography NAS folder to get started</p>
      <button class="btn-primary btn-large" on:click={selectFolder}>Select Root Folder</button>
    </div>
  {:else}
    <div class="toolbar">
      <button class="btn-icon" on:click={navigateBack} disabled={pathHistory.length === 0} title="Go back">←</button>
      <span class="path-display" title={currentPath}>{truncatePath(currentPath)}</span>
    </div>

    <div class="content-area">

      <!-- ── Left: file list ───────────────────────────────────────────────── -->
      <div class="file-list-pane" style={leftStyle}>
        {#if error}
          <div class="error-bar">{error}</div>
        {/if}
        {#if loading}
          <div class="status-row">Loading…</div>
        {:else if files.length === 0}
          <div class="status-row">This folder is empty</div>
        {:else}
          <table class="file-table">
            <thead>
              <tr>
                <th class="col-name">Name</th>
                <th class="col-date">Date Modified</th>
                <th class="col-size">Size</th>
              </tr>
            </thead>
            <tbody>
              {#each files as entry (entry.path)}
                <tr
                  class="file-row"
                  class:is-dir={entry.isDir}
                  class:is-fits={!entry.isDir && isFits(entry.name)}
                  class:selected={selectedEntry && selectedEntry.path === entry.path}
                  on:click={() => onRowClick(entry)}
                >
                  <td class="col-name">
                    <span class="file-icon">{entry.isDir ? '📁' : isFits(entry.name) ? '🔭' : '🗒'}</span>
                    <span class="file-name">{entry.name}</span>
                  </td>
                  <td class="col-date">{formatDate(entry.modTime)}</td>
                  <td class="col-size">{formatSize(entry.size, entry.isDir)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      <!-- ── Divider ───────────────────────────────────────────────────────── -->
      {#if selectedEntry}
        <div class="divider" on:mousedown={onDividerMouseDown}>
          <button
            class="collapse-btn"
            on:mousedown|stopPropagation
            on:click={() => collapsed = !collapsed}
            title={collapsed ? 'Expand file list' : 'Collapse file list'}
          >{collapsed ? '›' : '‹'}</button>
        </div>
      {/if}

      <!-- ── Right: preview ────────────────────────────────────────────────── -->
      {#if selectedEntry}
        <div class="preview-pane">

          <div class="preview-titlebar">
            <span class="preview-filename" title={selectedEntry.path}>{selectedEntry.name}</span>
            <div class="preview-controls">
              <span class="zoom-label">{Math.round(zoom * 100)}%</span>
              <button class="tool-btn" on:click={resetView} title="Fit to window (or double-click image)">Fit</button>
              <div class="stretch-group">
                <button
                  class="tool-btn"
                  class:active={stretchEnabled}
                  on:click={() => { stretchEnabled = !stretchEnabled; refreshPreview() }}
                  title="Toggle autostretch"
                >Stretch</button>
                {#if stretchEnabled}
                  <button class="tool-btn preset" class:active={stretchLevel === 1} on:click={() => setStretch(1)}>Gentle</button>
                  <button class="tool-btn preset" class:active={stretchLevel === 2} on:click={() => setStretch(2)}>Normal</button>
                  <button class="tool-btn preset" class:active={stretchLevel === 3} on:click={() => setStretch(3)}>Strong</button>
                {/if}
              </div>
              <button class="btn-icon small" on:click={clearPreview} title="Close preview">✕</button>
            </div>
          </div>

          <!-- Image viewport: overflow hidden, zoom/pan via transform -->
          <div
            class="image-viewport"
            class:panning={isPanning}
            on:wheel|preventDefault={onWheel}
            on:mousedown={onPanStart}
            on:mousemove={onPanMove}
            on:mouseup={onPanEnd}
            on:mouseleave={onPanEnd}
            on:dblclick={resetView}
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
                alt={selectedEntry.name}
                style="transform: translate({panX}px, {panY}px) scale({zoom});"
                draggable="false"
              />
            {/if}
          </div>

          {#if fitsHeader}
            <div class="preview-meta">
              <div class="meta-section">
                <button class="meta-section-hdr" on:click={() => basicCollapsed = !basicCollapsed}>
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
                <button class="meta-section-hdr" on:click={() => advancedCollapsed = !advancedCollapsed}>
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
      {/if}

    </div>

    <footer>
      <span>{files.length} item{files.length !== 1 ? 's' : ''}</span>
      <span class="root-tag">Root: {truncatePath(rootFolder, 50)}</span>
    </footer>
  {/if}
</div>

<style>
  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  /* ── Header / toolbar ────────────────────────────────────────────────────── */

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 20px;
    height: 52px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    -webkit-app-region: drag;
  }

  .logo {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--accent);
    letter-spacing: 0.05em;
  }

  .header-right { -webkit-app-region: no-drag; }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 16px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .path-display {
    font-size: 0.82rem;
    color: var(--text-secondary);
    font-family: 'Consolas', 'Fira Code', monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ── Buttons ─────────────────────────────────────────────────────────────── */

  .btn-primary {
    background: var(--accent-dim);
    color: var(--text-primary);
    border: 1px solid var(--accent);
    border-radius: 6px;
    padding: 6px 16px;
    font-size: 0.85rem;
    cursor: pointer;
    transition: background 0.15s;
  }

  .btn-primary:hover { background: var(--accent); color: #0f111a; }
  .btn-large { padding: 10px 28px; font-size: 1rem; margin-top: 16px; }

  .btn-icon {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    width: 28px;
    height: 28px;
    font-size: 1rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
    transition: color 0.15s, border-color 0.15s;
  }

  .btn-icon.small { width: 22px; height: 22px; font-size: 0.8rem; }
  .btn-icon:hover:not(:disabled) { color: var(--accent); border-color: var(--accent); }
  .btn-icon:disabled { opacity: 0.3; cursor: default; }

  .tool-btn {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 2px 8px;
    font-size: 0.75rem;
    cursor: pointer;
    transition: color 0.15s, border-color 0.15s;
  }
  .tool-btn:hover { color: var(--accent); border-color: var(--accent); }
  .tool-btn.active { color: var(--accent); border-color: var(--accent); background: var(--accent-dim); }
  .tool-btn.preset { padding: 2px 6px; }

  .stretch-group {
    display: flex;
    align-items: center;
    gap: 3px;
  }

  /* ── Two-pane content area ───────────────────────────────────────────────── */

  .content-area {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  .file-list-pane {
    overflow-y: auto;
    overflow-x: hidden;
    transition: flex 0.18s ease;
    min-width: 0;
  }

  .file-list-pane::-webkit-scrollbar { width: 6px; }
  .file-list-pane::-webkit-scrollbar-track { background: var(--bg-base); }
  .file-list-pane::-webkit-scrollbar-thumb { background: var(--border-accent); border-radius: 3px; }

  /* ── Resize divider ──────────────────────────────────────────────────────── */

  .divider {
    width: 5px;
    flex-shrink: 0;
    background: var(--border);
    cursor: col-resize;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    transition: background 0.15s;
  }

  .divider:hover { background: var(--accent-dim); }

  .collapse-btn {
    position: absolute;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 50%;
    width: 18px;
    height: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    font-size: 0.7rem;
    color: var(--text-secondary);
    padding: 0;
    line-height: 1;
    z-index: 10;
    transition: color 0.15s, border-color 0.15s;
  }

  .collapse-btn:hover { color: var(--accent); border-color: var(--accent); }

  /* ── File table ──────────────────────────────────────────────────────────── */

  .file-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
  }

  .file-table thead tr {
    background: var(--bg-panel);
    position: sticky;
    top: 0;
    z-index: 1;
  }

  .file-table th {
    padding: 8px 16px;
    text-align: left;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-dim);
    border-bottom: 1px solid var(--border);
  }

  .file-row td {
    padding: 7px 16px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
  }

  .file-row:hover td { background: var(--bg-row-hover); }
  .file-row.is-dir  { cursor: pointer; }
  .file-row.is-fits { cursor: pointer; }

  .file-row.is-dir td { background: var(--bg-row-dir); color: var(--accent); }
  .file-row.is-dir:hover td,
  .file-row.is-fits:hover td { background: var(--bg-row-hover); }

  .file-row.selected td { background: var(--accent-dim) !important; color: var(--text-primary); }

  .col-name { width: 55%; }
  .col-date { width: 30%; color: var(--text-secondary); font-size: 0.82rem; font-variant-numeric: tabular-nums; }
  .col-size { width: 15%; text-align: right; color: var(--text-secondary); font-size: 0.82rem; font-variant-numeric: tabular-nums; padding-right: 24px !important; }
  .file-icon { margin-right: 8px; font-size: 0.9em; }

  /* ── Preview pane ────────────────────────────────────────────────────────── */

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

  /* ── Image viewport (zoom/pan container) ─────────────────────────────────── */

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

  /* ── FITS metadata (scrollable strip at bottom) ───────────────────────────── */

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

  /* ── Misc ────────────────────────────────────────────────────────────────── */

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .empty-icon { font-size: 3rem; color: var(--accent-dim); margin-bottom: 8px; }
  .empty-title { font-size: 1.1rem; font-weight: 500; color: var(--text-primary); }
  .empty-sub { font-size: 0.875rem; color: var(--text-secondary); max-width: 340px; text-align: center; }

  .status-row { padding: 32px; text-align: center; color: var(--text-dim); font-size: 0.875rem; }

  .error-bar {
    background: #2a1020;
    border: 1px solid var(--danger);
    color: var(--danger);
    padding: 8px 16px;
    font-size: 0.82rem;
    border-radius: 4px;
    margin: 8px 16px;
  }

  footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 16px;
    background: var(--bg-panel);
    border-top: 1px solid var(--border);
    font-size: 0.75rem;
    color: var(--text-dim);
    flex-shrink: 0;
  }

  .root-tag { font-family: 'Consolas', 'Fira Code', monospace; color: var(--text-dim); }
</style>
