<script>
  import { SelectRootFolder, ListDirectory } from '../wailsjs/go/app/App.js'

  let rootFolder = ''
  let currentPath = ''
  let pathHistory = []
  let files = []
  let error = ''
  let loading = false

  async function selectFolder() {
    const path = await SelectRootFolder()
    if (path) {
      rootFolder = path
      pathHistory = []
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

  async function navigateInto(entry) {
    if (!entry.isDir) return
    pathHistory = [...pathHistory, currentPath]
    await loadDirectory(entry.path)
  }

  async function navigateBack() {
    if (pathHistory.length === 0) return
    const prev = pathHistory[pathHistory.length - 1]
    pathHistory = pathHistory.slice(0, -1)
    await loadDirectory(prev)
  }

  function formatDate(dateStr) {
    const d = new Date(dateStr)
    return d.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
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
</script>

<div class="layout">
  <header>
    <div class="header-left">
      <span class="logo">✦ Eirin</span>
    </div>
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
      <button
        class="btn-icon"
        on:click={navigateBack}
        disabled={pathHistory.length === 0}
        title="Go back"
      >←</button>
      <span class="path-display" title={currentPath}>{truncatePath(currentPath)}</span>
    </div>

    <div class="file-list-container">
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
                on:click={() => navigateInto(entry)}
                style={entry.isDir ? 'cursor:pointer' : 'cursor:default'}
              >
                <td class="col-name">
                  <span class="file-icon">{entry.isDir ? '📁' : '🗒'}</span>
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

  .file-list-container {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
  }

  .file-list-container::-webkit-scrollbar {
    width: 6px;
  }

  .file-list-container::-webkit-scrollbar-track {
    background: var(--bg-base);
  }

  .file-list-container::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

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

  .file-row.is-dir td {
    background: var(--bg-row-dir);
    color: var(--accent);
  }

  .file-row.is-dir:hover td {
    background: var(--bg-row-hover);
  }

  .col-name {
    width: 55%;
  }

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

  .file-icon {
    margin-right: 8px;
    font-size: 0.9em;
  }

  .file-name {
    vertical-align: middle;
  }

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
