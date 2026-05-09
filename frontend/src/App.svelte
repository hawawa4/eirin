<script>
  import {
    SelectRootFolder,
    ListDirectory,
    GeneratePreview,
    ReadFITSHeader,
  } from '../wailsjs/go/app/App.js'

  let rootFolder = ''
  let currentPath = ''
  let pathHistory = []
  let files = []
  let error = ''
  let loading = false

  let selectedEntry = null
  let previewDataUrl = ''
  let previewLoading = false
  let previewError = ''
  let fitsHeader = null

  function isFits(name) {
    const l = name.toLowerCase()
    return l.endsWith('.fits') || l.endsWith('.fit')
  }

  async function selectFolder() {
    const path = await SelectRootFolder()
    if (path) {
      rootFolder = path
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

    const [hdrResult, imgResult] = await Promise.allSettled([
      ReadFITSHeader(entry.path),
      GeneratePreview(entry.path),
    ])

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
  }

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

  function formatExpTime(sec) {
    if (!sec) return '—'
    if (sec >= 60) return (sec / 60).toFixed(1) + ' min'
    return sec + ' s'
  }

  function metaRows(h) {
    if (!h) return []
    return [
      { key: 'Object',       val: h.object      || '—' },
      { key: 'Filter',       val: h.filter       || '—' },
      { key: 'Exposure',     val: formatExpTime(h.exptime) },
      { key: 'Date',         val: h.dateObs      || '—' },
      { key: 'Gain',         val: h.gain         ? h.gain  : '—' },
      { key: 'CCD Temp',     val: h.ccdTemp      ? h.ccdTemp + ' °C' : '—' },
      { key: 'Telescope',    val: h.telescope    || '—' },
      { key: 'Camera',       val: h.instrument   || '—' },
      { key: 'Size',         val: h.width && h.height ? `${h.width} × ${h.height}${h.channels > 1 ? ` × ${h.channels}` : ''}` : '—' },
      { key: 'Binning',      val: h.xbinning     ? `${h.xbinning} × ${h.ybinning}` : '—' },
    ]
  }
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
      <!-- ── File list ─────────────────────────────────────────────────────── -->
      <div class="file-list-pane" class:narrowed={selectedEntry}>
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
                    <span class="file-icon">
                      {entry.isDir ? '📁' : isFits(entry.name) ? '🔭' : '🗒'}
                    </span>
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

      <!-- ── Preview panel ──────────────────────────────────────────────────── -->
      {#if selectedEntry}
        <div class="preview-pane">
          <div class="preview-titlebar">
            <span class="preview-filename" title={selectedEntry.path}>{selectedEntry.name}</span>
            <button class="btn-icon" on:click={clearPreview} title="Close preview">✕</button>
          </div>

          <div class="preview-image-area">
            {#if previewLoading}
              <div class="preview-status">
                <span class="spinner">◌</span>
                Generating preview…
              </div>
            {:else if previewError}
              <div class="preview-error">{previewError}</div>
            {:else if previewDataUrl}
              <img class="preview-img" src={previewDataUrl} alt={selectedEntry.name} />
            {/if}
          </div>

          {#if fitsHeader}
            <div class="preview-meta">
              <div class="meta-title">FITS Header</div>
              {#each metaRows(fitsHeader) as row}
                <div class="meta-row">
                  <span class="meta-key">{row.key}</span>
                  <span class="meta-val">{row.val}</span>
                </div>
              {/each}
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

  .header-right {
    -webkit-app-region: no-drag;
  }

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

  .btn-primary:hover {
    background: var(--accent);
    color: #0f111a;
  }

  .btn-large {
    padding: 10px 28px;
    font-size: 1rem;
    margin-top: 16px;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 16px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

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
    transition: color 0.15s, border-color 0.15s;
    flex-shrink: 0;
  }

  .btn-icon:hover:not(:disabled) {
    color: var(--accent);
    border-color: var(--accent);
  }

  .btn-icon:disabled {
    opacity: 0.3;
    cursor: default;
  }

  .path-display {
    font-size: 0.82rem;
    color: var(--text-secondary);
    font-family: 'Consolas', 'Fira Code', monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ── Two-pane content area ───────────────────────────────────────────────── */

  .content-area {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  .file-list-pane {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    transition: flex 0.2s ease;
  }

  .file-list-pane.narrowed {
    flex: 0 0 42%;
    border-right: 1px solid var(--border);
  }

  .file-list-pane::-webkit-scrollbar,
  .preview-pane::-webkit-scrollbar {
    width: 6px;
  }

  .file-list-pane::-webkit-scrollbar-track,
  .preview-pane::-webkit-scrollbar-track {
    background: var(--bg-base);
  }

  .file-list-pane::-webkit-scrollbar-thumb,
  .preview-pane::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

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

  .file-row:hover td {
    background: var(--bg-row-hover);
  }

  .file-row.is-dir { cursor: pointer; }
  .file-row.is-fits { cursor: pointer; }

  .file-row.is-dir td {
    background: var(--bg-row-dir);
    color: var(--accent);
  }

  .file-row.is-dir:hover td,
  .file-row.is-fits:hover td {
    background: var(--bg-row-hover);
  }

  .file-row.selected td {
    background: var(--accent-dim) !important;
    color: var(--text-primary);
  }

  .col-name { width: 55%; }

  .col-date {
    width: 30%;
    color: var(--text-secondary);
    font-size: 0.82rem;
    font-variant-numeric: tabular-nums;
  }

  .col-size {
    width: 15%;
    text-align: right;
    color: var(--text-secondary);
    font-size: 0.82rem;
    font-variant-numeric: tabular-nums;
    padding-right: 24px !important;
  }

  .file-icon { margin-right: 8px; font-size: 0.9em; }
  .file-name { vertical-align: middle; }

  /* ── Preview pane ────────────────────────────────────────────────────────── */

  .preview-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    background: var(--bg-base);
  }

  .preview-titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 14px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .preview-filename {
    font-size: 0.82rem;
    font-family: 'Consolas', 'Fira Code', monospace;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .preview-image-area {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 16px;
    min-height: 200px;
  }

  .preview-img {
    max-width: 100%;
    max-height: 55vh;
    object-fit: contain;
    border-radius: 4px;
    border: 1px solid var(--border);
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

  @keyframes spin {
    from { transform: rotate(0deg); }
    to   { transform: rotate(360deg); }
  }

  .preview-error {
    color: var(--danger);
    font-size: 0.82rem;
    background: #2a1020;
    border: 1px solid var(--danger);
    border-radius: 4px;
    padding: 10px 14px;
    width: 100%;
  }

  /* ── FITS metadata ───────────────────────────────────────────────────────── */

  .preview-meta {
    padding: 0 16px 16px;
    flex-shrink: 0;
  }

  .meta-title {
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-dim);
    margin-bottom: 8px;
    padding-top: 4px;
    border-top: 1px solid var(--border);
  }

  .meta-row {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    padding: 3px 0;
    font-size: 0.8rem;
    border-bottom: 1px solid var(--border);
  }

  .meta-key {
    color: var(--text-dim);
    flex-shrink: 0;
    width: 90px;
  }

  .meta-val {
    color: var(--text-primary);
    text-align: right;
    font-variant-numeric: tabular-nums;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: calc(100% - 98px);
  }

  /* ── Misc ────────────────────────────────────────────────────────────────── */

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--text-secondary);
  }

  .empty-icon {
    font-size: 3rem;
    color: var(--accent-dim);
    margin-bottom: 8px;
  }

  .empty-title {
    font-size: 1.1rem;
    font-weight: 500;
    color: var(--text-primary);
  }

  .empty-sub {
    font-size: 0.875rem;
    color: var(--text-secondary);
    max-width: 340px;
    text-align: center;
  }

  .status-row {
    padding: 32px;
    text-align: center;
    color: var(--text-dim);
    font-size: 0.875rem;
  }

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

  .root-tag {
    font-family: 'Consolas', 'Fira Code', monospace;
    color: var(--text-dim);
  }
</style>
