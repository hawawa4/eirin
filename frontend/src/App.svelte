<script lang="ts">
  import { onMount } from 'svelte'
  import {
    SelectRootFolder,
    ListDirectoryEnriched,
    BuildIndex,
    CancelIndex,
    GeneratePreview,
    ReadFITSHeader,
    LoadPrefs,
    SetPref,
  } from '../wailsjs/go/app/App.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import type { browser, fits } from '../wailsjs/go/models'

  const PREF_ROOT_FOLDER        = 'root_folder'
  const PREF_BASIC_COLLAPSED    = 'basic_collapsed'
  const PREF_ADVANCED_COLLAPSED = 'advanced_collapsed'
  const PREF_STRETCH_ENABLED    = 'stretch_enabled'
  const PREF_STRETCH_LEVEL      = 'stretch_level'
  const PREF_COLUMN_CONFIG      = 'column_config'

  // ── Column config types ───────────────────────────────────────────────────
  interface ColumnDef {
    id:      string
    label:   string
    visible: boolean
    width:   number
    order:   number
  }

  interface FileGroup {
    key:    string
    object: string
    date:   string
    files:  browser.EnrichedFileEntry[]
  }

  interface IndexProgress {
    phase:   'scanning' | 'indexing' | 'done' | 'cancelled'
    total:   number
    done:    number
    indexed: number
    errors:  number
    current: string
  }

  const DEFAULT_COLUMNS: ColumnDef[] = [
    { id: 'name',    label: 'Name',     visible: true,  width: 160, order: 0 },
    { id: 'object',  label: 'Object',   visible: true,  width: 110, order: 1 },
    { id: 'filter',  label: 'Filter',   visible: true,  width: 70,  order: 2 },
    { id: 'dateObs', label: 'Date',     visible: true,  width: 130, order: 3 },
    { id: 'expTime', label: 'Exp',      visible: true,  width: 65,  order: 4 },
    { id: 'modTime', label: 'Modified', visible: false, width: 140, order: 5 },
    { id: 'size',    label: 'Size',     visible: false, width: 70,  order: 6 },
    { id: 'gain',    label: 'Gain',     visible: false, width: 60,  order: 7 },
    { id: 'ccdTemp', label: 'Temp',     visible: false, width: 75,  order: 8 },
  ]

  // ── File browser state ────────────────────────────────────────────────────
  let rootFolder  = $state('')
  let currentPath = $state('')
  let pathHistory = $state<string[]>([])
  let files       = $state<browser.EnrichedFileEntry[]>([])
  let error       = $state('')
  let loading     = $state(false)

  // ── Toolbar state ─────────────────────────────────────────────────────────
  let searchQuery   = $state('')
  let filterFilter  = $state('')
  let groupByObject = $state(false)
  let showColumnMenu = $state(false)

  // ── Column config ─────────────────────────────────────────────────────────
  let columns = $state<ColumnDef[]>(DEFAULT_COLUMNS.map(c => ({ ...c })))

  // ── Index builder state ───────────────────────────────────────────────────
  let indexProgress = $state<IndexProgress | null>(null)
  let indexRunning  = $derived(
    indexProgress !== null &&
    indexProgress.phase !== 'done' &&
    indexProgress.phase !== 'cancelled'
  )

  // ── Preview state ─────────────────────────────────────────────────────────
  let selectedEntry  = $state<browser.EnrichedFileEntry | null>(null)
  let previewDataUrl = $state('')
  let previewLoading = $state(false)
  let previewError   = $state('')
  let fitsHeader     = $state<fits.FITSHeader | null>(null)

  // ── Stretch controls ──────────────────────────────────────────────────────
  let stretchEnabled = $state(true)
  let stretchLevel   = $state(2)

  // ── Header section collapse ───────────────────────────────────────────────
  let basicCollapsed    = $state(false)
  let advancedCollapsed = $state(true)

  // ── Preference persistence guard ──────────────────────────────────────────
  let prefsLoaded = $state(false)

  // ── Preview request ID (stale cancellation, non-reactive) ─────────────────
  let previewReqId = 0

  // ── Column drag state (non-reactive) ─────────────────────────────────────
  let dragSourceId  = ''
  let dragOverIndex = $state(-1)

  onMount(async () => {
    const p = await LoadPrefs()
    stretchEnabled    = p.stretchEnabled
    stretchLevel      = p.stretchLevel
    basicCollapsed    = p.basicCollapsed
    advancedCollapsed = p.advancedCollapsed

    if (p.columnConfig) {
      try {
        const saved = JSON.parse(p.columnConfig) as ColumnDef[]
        // Merge saved into defaults so new columns get their defaults.
        columns = DEFAULT_COLUMNS.map(def => {
          const s = saved.find(x => x.id === def.id)
          return s ? { ...def, ...s } : { ...def }
        })
      } catch (_) { /* keep defaults */ }
    }

    if (p.rootFolder) {
      rootFolder = p.rootFolder
      await loadDirectory(p.rootFolder)
    }
    prefsLoaded = true

    // Listen for index progress events from the Go backend.
    EventsOn('index:progress', (data: IndexProgress) => {
      indexProgress = data
      if ((data.phase === 'done' || data.phase === 'cancelled') && currentPath) {
        // Reload current dir so newly indexed metadata becomes visible.
        setTimeout(() => loadDirectory(currentPath), 400)
      }
    })
  })

  // Auto-save stretch/header prefs when they change.
  $effect(() => { if (prefsLoaded) SetPref(PREF_STRETCH_ENABLED,    String(stretchEnabled)) })
  $effect(() => { if (prefsLoaded) SetPref(PREF_STRETCH_LEVEL,      String(stretchLevel)) })
  $effect(() => { if (prefsLoaded) SetPref(PREF_BASIC_COLLAPSED,    String(basicCollapsed)) })
  $effect(() => { if (prefsLoaded) SetPref(PREF_ADVANCED_COLLAPSED, String(advancedCollapsed)) })

  function saveColumnConfig() {
    if (!prefsLoaded) return
    SetPref(PREF_COLUMN_CONFIG, JSON.stringify(columns))
  }

  async function startBuildIndex() {
    if (!rootFolder || indexRunning) return
    indexProgress = { phase: 'scanning', total: 0, done: 0, indexed: 0, errors: 0, current: 'Starting…' }
    await BuildIndex(rootFolder)  // returns immediately; progress via events
  }

  async function cancelBuildIndex() {
    await CancelIndex()
  }

  // Close column menu when clicking outside it.
  $effect(() => {
    if (!showColumnMenu) return
    function onDoc(e: MouseEvent) {
      const el = document.getElementById('column-menu-root')
      if (el && !el.contains(e.target as Node)) showColumnMenu = false
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  })

  // ── Derived ───────────────────────────────────────────────────────────────
  let visibleColumns = $derived(
    [...columns].filter(c => c.visible).sort((a, b) => a.order - b.order)
  )

  let totalColWidth = $derived(visibleColumns.reduce((s, c) => s + c.width, 0))

  let uniqueFilters = $derived(
    [...new Set(
      files.filter(f => !f.isDir && f.filter).map(f => f.filter)
    )].sort()
  )

  let filteredFiles = $derived(applyFilters(files, searchQuery, filterFilter))
  let filteredDirs  = $derived(filteredFiles.filter(f => f.isDir))
  let filteredFits  = $derived(filteredFiles.filter(f => !f.isDir && f.hasMeta))
  let filteredOther = $derived(filteredFiles.filter(f => !f.isDir && !f.hasMeta))
  let fileGroups    = $derived(groupByObject ? buildGroups(filteredFits) : null)

  let totalCount    = $derived(files.length)
  let filteredCount = $derived(filteredFiles.length)
  let uncachedCount = $derived(
    files.filter(f => !f.isDir && isFits(f.name) && !f.hasMeta).length
  )

  function applyFilters(
    all: browser.EnrichedFileEntry[],
    search: string,
    filter: string,
  ): browser.EnrichedFileEntry[] {
    let r = all
    if (search) {
      const q = search.toLowerCase()
      r = r.filter(f =>
        f.name.toLowerCase().includes(q) ||
        (f.object && f.object.toLowerCase().includes(q))
      )
    }
    if (filter) {
      r = r.filter(f => f.isDir || f.filter === filter)
    }
    return r
  }

  function buildGroups(fits: browser.EnrichedFileEntry[]): FileGroup[] {
    const map = new Map<string, FileGroup>()
    for (const f of fits) {
      const obj  = f.object  || '—'
      const date = f.dateObs ? f.dateObs.substring(0, 10) : '—'
      const key  = `${obj}|${date}`
      if (!map.has(key)) map.set(key, { key, object: obj, date, files: [] })
      map.get(key)!.files.push(f)
    }
    return [...map.values()].sort((a, b) =>
      a.object !== b.object
        ? a.object.localeCompare(b.object)
        : a.date.localeCompare(b.date)
    )
  }

  // ── Pane resize ───────────────────────────────────────────────────────────
  let leftPct   = $state(42)
  let collapsed = $state(false)

  function onDividerMouseDown(e: MouseEvent) {
    e.preventDefault()
    const contentArea = document.querySelector('.content-area') as HTMLElement

    function onMove(ev: MouseEvent) {
      const rect = contentArea.getBoundingClientRect()
      const pct = ((ev.clientX - rect.left) / rect.width) * 100
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

  // ── Column resize ─────────────────────────────────────────────────────────
  function startColResize(e: MouseEvent, colId: string) {
    e.preventDefault()
    e.stopPropagation()
    const startX = e.clientX
    const col = columns.find(c => c.id === colId)!
    const startWidth = col.width

    function onMove(ev: MouseEvent) {
      const c = columns.find(c => c.id === colId)!
      c.width = Math.max(48, startWidth + ev.clientX - startX)
    }
    function onUp() {
      saveColumnConfig()
      document.removeEventListener('mousemove', onMove)
      document.removeEventListener('mouseup', onUp)
    }
    document.addEventListener('mousemove', onMove)
    document.addEventListener('mouseup', onUp)
  }

  // ── Column drag-and-drop reorder ──────────────────────────────────────────
  function onColDragStart(e: DragEvent, visIdx: number) {
    dragSourceId = visibleColumns[visIdx].id
    e.dataTransfer!.effectAllowed = 'move'
  }

  function onColDragOver(e: DragEvent, visIdx: number) {
    e.preventDefault()
    e.dataTransfer!.dropEffect = 'move'
    dragOverIndex = visIdx
  }

  function onColDrop(e: DragEvent, targetVisIdx: number) {
    e.preventDefault()
    dragOverIndex = -1
    if (!dragSourceId) return

    const srcVisIdx = visibleColumns.findIndex(c => c.id === dragSourceId)
    dragSourceId = ''
    if (srcVisIdx === -1 || srcVisIdx === targetVisIdx) return

    // Rebuild visible order, then reassign orders globally.
    const newVis = [...visibleColumns]
    const [moved] = newVis.splice(srcVisIdx, 1)
    newVis.splice(targetVisIdx, 0, moved)

    const invisible = columns.filter(c => !c.visible)
    let order = 0
    for (const col of newVis)     columns.find(c => c.id === col.id)!.order = order++
    for (const col of invisible)  columns.find(c => c.id === col.id)!.order = order++

    saveColumnConfig()
  }

  function onColDragEnd() { dragSourceId = ''; dragOverIndex = -1 }

  function toggleColumn(colId: string) {
    const col = columns.find(c => c.id === colId)!
    col.visible = !col.visible
    saveColumnConfig()
  }

  // ── Zoom / pan ────────────────────────────────────────────────────────────
  let zoom      = $state(1)
  let panX      = $state(0)
  let panY      = $state(0)
  let isPanning = $state(false)
  let panStartX = 0
  let panStartY = 0

  function resetView() { zoom = 1; panX = 0; panY = 0 }

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

  // ── Navigation ────────────────────────────────────────────────────────────
  function isFits(name: string): boolean {
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

  async function loadDirectory(path: string) {
    loading = true
    error   = ''
    searchQuery  = ''
    filterFilter = ''
    try {
      const result = await ListDirectoryEnriched(path)
      files = (result || []).slice().sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
        return a.name.localeCompare(b.name)
      })
      currentPath = path
    } catch (e) {
      error = String(e)
      files = []
    } finally {
      loading = false
    }
  }

  async function onRowClick(entry: browser.EnrichedFileEntry) {
    if (entry.isDir) {
      clearPreview()
      pathHistory = [...pathHistory, currentPath]
      await loadDirectory(entry.path)
    } else if (isFits(entry.name)) {
      await openPreview(entry)
    }
  }

  async function openPreview(entry: browser.EnrichedFileEntry) {
    selectedEntry  = entry
    previewDataUrl = ''
    previewError   = ''
    fitsHeader     = null
    previewLoading = true
    resetView()

    const id    = ++previewReqId
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
      previewError = (imgResult as PromiseRejectedResult).reason?.toString() ?? 'Preview failed'
    }
  }

  async function navigateBack() {
    if (pathHistory.length === 0) return
    const prev  = pathHistory[pathHistory.length - 1]
    pathHistory = pathHistory.slice(0, -1)
    clearPreview()
    await loadDirectory(prev)
  }

  function clearPreview() {
    selectedEntry  = null
    previewDataUrl = ''
    previewError   = ''
    fitsHeader     = null
    resetView()
  }

  async function refreshPreview() {
    if (!selectedEntry) return
    const id    = ++previewReqId
    previewLoading = true
    previewError   = ''
    const level = stretchEnabled ? stretchLevel : 0
    try {
      const result = await GeneratePreview(selectedEntry.path, level)
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

  // ── Formatting helpers ────────────────────────────────────────────────────
  function formatDate(dateStr: string): string {
    const d = new Date(dateStr)
    return d.toLocaleDateString('en-US', {
      year: 'numeric', month: 'short', day: '2-digit',
      hour: '2-digit', minute: '2-digit',
    })
  }

  function formatExpTime(secs: number): string {
    if (!secs) return '—'
    if (secs >= 60) return (secs / 60).toFixed(1) + ' min'
    return secs + ' s'
  }

  function formatSize(bytes: number, isDir: boolean): string {
    if (isDir) return '—'
    if (bytes < 1024) return bytes + ' B'
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
    if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
    return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
  }

  function truncatePath(path: string, maxLen = 60): string {
    if (path.length <= maxLen) return path
    const parts = path.split('/')
    if (parts.length <= 2) return '…' + path.slice(-(maxLen - 1))
    return '…/' + parts.slice(-2).join('/')
  }

  function getCellValue(entry: browser.EnrichedFileEntry, colId: string): string {
    switch (colId) {
      case 'name':    return entry.name
      case 'object':  return entry.hasMeta ? (entry.object  || '—') : (entry.isDir ? '—' : '')
      case 'filter':  return entry.hasMeta ? (entry.filter  || '—') : (entry.isDir ? '—' : '')
      case 'dateObs': return entry.hasMeta ? (entry.dateObs ? entry.dateObs.substring(0, 10) : '—') : (entry.isDir ? '—' : '')
      case 'expTime': return entry.hasMeta ? formatExpTime(entry.expTime) : (entry.isDir ? '—' : '')
      case 'modTime': return formatDate(entry.modTime)
      case 'size':    return formatSize(entry.size, entry.isDir)
      case 'gain':    return entry.hasMeta ? (entry.gain    ? String(entry.gain)             : '—') : ''
      case 'ccdTemp': return entry.hasMeta ? (entry.ccdTemp ? entry.ccdTemp.toFixed(1) + ' °C' : '—') : ''
      default:        return '—'
    }
  }

  interface MetaRow { key: string; val: string }

  function basicRows(h: fits.FITSHeader | null): MetaRow[] {
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

  function advancedRows(h: fits.FITSHeader | null): MetaRow[] {
    if (!h) return []
    return [
      { key: 'Gain',      val: h.gain       ? String(h.gain) : '—' },
      { key: 'CCD Temp',  val: h.ccdTemp    ? h.ccdTemp + ' °C' : '—' },
      { key: 'Telescope', val: h.telescope  || '—' },
      { key: 'Camera',    val: h.instrument || '—' },
      { key: 'Binning',   val: h.xbinning ? `${h.xbinning} × ${h.ybinning}` : '—' },
    ]
  }

  let leftStyle = $derived(
    selectedEntry
      ? collapsed
        ? 'flex: 0 0 0px; min-width: 0; overflow: hidden;'
        : `flex: 0 0 ${leftPct}%;`
      : 'flex: 1;'
  )
</script>

<div class="layout">
  <header>
    <span class="logo">✦ Eirin</span>
    <div class="header-right">
      {#if rootFolder}
        <button
          class="btn-secondary"
          onclick={startBuildIndex}
          disabled={indexRunning}
          title="Scan all subfolders and index FITS headers. Re-run to pick up new files."
        >
          {indexRunning ? 'Indexing…' : 'Build Index'}
        </button>
      {/if}
      <button class="btn-primary" onclick={selectFolder}>
        {rootFolder ? 'Change Root Folder' : 'Select Root Folder'}
      </button>
    </div>
  </header>

  {#if !rootFolder}
    <div class="empty-state">
      <div class="empty-icon">◎</div>
      <p class="empty-title">No folder selected</p>
      <p class="empty-sub">Choose your astrophotography NAS folder to get started</p>
      <button class="btn-primary btn-large" onclick={selectFolder}>Select Root Folder</button>
    </div>
  {:else}
    <div class="toolbar">
      <button class="btn-icon" onclick={navigateBack} disabled={pathHistory.length === 0} title="Go back">←</button>
      <span class="path-display" title={currentPath}>{truncatePath(currentPath)}</span>
    </div>

    {#if indexProgress && indexProgress.phase !== 'done' && indexProgress.phase !== 'cancelled'}
      <div class="index-bar">
        <span class="index-phase">
          {#if indexProgress.phase === 'scanning'}
            Scanning directories…
          {:else}
            Indexing {indexProgress.done} / {indexProgress.total}
            {#if indexProgress.indexed > 0}
              <span class="index-new">+{indexProgress.indexed} new</span>
            {/if}
          {/if}
        </span>
        <div class="index-track">
          <div
            class="index-fill"
            style="width: {indexProgress.total > 0 ? (indexProgress.done / indexProgress.total * 100).toFixed(1) : 0}%"
          ></div>
        </div>
        <span class="index-file" title={indexProgress.current}>{indexProgress.current}</span>
        <button class="tool-btn" onclick={cancelBuildIndex}>Cancel</button>
      </div>
    {/if}

    <div class="content-area">

      <!-- ── Left: file list ───────────────────────────────────────────────── -->
      <div class="file-list-pane" style={leftStyle}>

        <!-- Toolbar: search, filter, group, columns -->
        <div class="table-toolbar">
          <input
            class="search-input"
            type="search"
            placeholder="Search name or object…"
            bind:value={searchQuery}
          />
          {#if uniqueFilters.length > 0}
            <select class="filter-select" bind:value={filterFilter}>
              <option value="">All filters</option>
              {#each uniqueFilters as f}
                <option value={f}>{f}</option>
              {/each}
            </select>
          {/if}
          <button
            class="tool-btn"
            class:active={groupByObject}
            onclick={() => (groupByObject = !groupByObject)}
            title="Group by object + date"
          >Group</button>
          <div class="column-selector" id="column-menu-root">
            <button
              class="tool-btn"
              onclick={() => (showColumnMenu = !showColumnMenu)}
              title="Show/hide columns"
            >Cols ▾</button>
            {#if showColumnMenu}
              <div class="column-menu">
                {#each [...columns].sort((a, b) => a.order - b.order) as col}
                  {#if col.id !== 'name'}
                    <label class="column-menu-item">
                      <input
                        type="checkbox"
                        checked={col.visible}
                        onchange={() => toggleColumn(col.id)}
                      />
                      {col.label}
                    </label>
                  {/if}
                {/each}
              </div>
            {/if}
          </div>
        </div>

        {#if error}
          <div class="error-bar">{error}</div>
        {/if}

        {#if loading}
          <div class="status-row">Loading…</div>
        {:else if files.length === 0}
          <div class="status-row">This folder is empty</div>
        {:else}
          <div class="table-scroll-wrapper">
            <table
              class="file-table"
              style="width: {Math.max(totalColWidth, 100)}px; min-width: 100%"
            >
              <colgroup>
                {#each visibleColumns as col}
                  <col style="width: {col.width}px" />
                {/each}
              </colgroup>
              <thead>
                <tr>
                  {#each visibleColumns as col, i}
                    <th
                      class:drag-over={dragOverIndex === i}
                      draggable={col.id !== 'name'}
                      ondragstart={(e) => onColDragStart(e, i)}
                      ondragover={(e) => onColDragOver(e, i)}
                      ondrop={(e) => onColDrop(e, i)}
                      ondragend={onColDragEnd}
                      ondragleave={() => { if (dragOverIndex === i) dragOverIndex = -1 }}
                    >
                      <span class="th-text">{col.label}</span>
                      {#if col.id !== 'name'}
                        <span
                          class="resize-handle"
                          onmousedown={(e) => startColResize(e, col.id)}
                          role="separator"
                          aria-label="Resize column"
                        ></span>
                      {/if}
                    </th>
                  {/each}
                </tr>
              </thead>
              <tbody>
                {#if !groupByObject}
                  <!-- Flat view: dirs first, then all files -->
                  {#each filteredFiles as entry (entry.path)}
                    <tr
                      class="file-row"
                      class:is-dir={entry.isDir}
                      class:is-fits={!entry.isDir && isFits(entry.name)}
                      class:selected={selectedEntry && selectedEntry.path === entry.path}
                      onclick={() => onRowClick(entry)}
                    >
                      {#each visibleColumns as col}
                        <td class="col-{col.id}">
                          {#if col.id === 'name'}
                            <span class="file-icon">{entry.isDir ? '📁' : isFits(entry.name) ? '🔭' : '🗒'}</span>
                            <span class="file-name">{entry.name}</span>
                          {:else}
                            {getCellValue(entry, col.id)}
                          {/if}
                        </td>
                      {/each}
                    </tr>
                  {/each}
                {:else}
                  <!-- Grouped view: dirs, then ungrouped files, then groups -->
                  {#each filteredDirs as entry (entry.path)}
                    <tr
                      class="file-row is-dir"
                      onclick={() => onRowClick(entry)}
                    >
                      {#each visibleColumns as col}
                        <td class="col-{col.id}">
                          {#if col.id === 'name'}
                            <span class="file-icon">📁</span>
                            <span class="file-name">{entry.name}</span>
                          {:else}
                            {getCellValue(entry, col.id)}
                          {/if}
                        </td>
                      {/each}
                    </tr>
                  {/each}

                  {#each filteredOther as entry (entry.path)}
                    <tr
                      class="file-row"
                      class:is-fits={isFits(entry.name)}
                      class:selected={selectedEntry && selectedEntry.path === entry.path}
                      onclick={() => onRowClick(entry)}
                    >
                      {#each visibleColumns as col}
                        <td class="col-{col.id}">
                          {#if col.id === 'name'}
                            <span class="file-icon">{isFits(entry.name) ? '🔭' : '🗒'}</span>
                            <span class="file-name">{entry.name}</span>
                          {:else}
                            {getCellValue(entry, col.id)}
                          {/if}
                        </td>
                      {/each}
                    </tr>
                  {/each}

                  {#each fileGroups ?? [] as group (group.key)}
                    <tr class="group-header-row">
                      <td colspan={visibleColumns.length}>
                        <span class="group-object">{group.object}</span>
                        <span class="group-date">{group.date}</span>
                        <span class="group-count">{group.files.length} frame{group.files.length !== 1 ? 's' : ''}</span>
                      </td>
                    </tr>
                    {#each group.files as entry (entry.path)}
                      <tr
                        class="file-row is-fits"
                        class:selected={selectedEntry && selectedEntry.path === entry.path}
                        onclick={() => onRowClick(entry)}
                      >
                        {#each visibleColumns as col}
                          <td class="col-{col.id}">
                            {#if col.id === 'name'}
                              <span class="file-icon">🔭</span>
                              <span class="file-name">{entry.name}</span>
                            {:else}
                              {getCellValue(entry, col.id)}
                            {/if}
                          </td>
                        {/each}
                      </tr>
                    {/each}
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <!-- ── Divider ───────────────────────────────────────────────────────── -->
      {#if selectedEntry}
        <div class="divider" onmousedown={onDividerMouseDown}>
          <button
            class="collapse-btn"
            onmousedown={(e) => e.stopPropagation()}
            onclick={() => (collapsed = !collapsed)}
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
              <button class="btn-icon small" onclick={clearPreview} title="Close preview">✕</button>
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
                alt={selectedEntry.name}
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
      {/if}

    </div>

    <footer>
      <span>
        {#if filteredCount !== totalCount}
          {filteredCount} of {totalCount} items
        {:else}
          {totalCount} item{totalCount !== 1 ? 's' : ''}
        {/if}
        {#if uncachedCount > 0 && !indexRunning}
          <span class="unindexed-hint" title="Click 'Build Index' to populate FITS metadata">
            · {uncachedCount} not indexed
          </span>
        {/if}
        {#if indexProgress?.phase === 'done'}
          <span class="index-done-hint">· Index up to date</span>
        {/if}
      </span>
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
    white-space: nowrap;
  }
  .tool-btn:hover { color: var(--accent); border-color: var(--accent); }
  .tool-btn.active { color: var(--accent); border-color: var(--accent); background: var(--accent-dim); }
  .tool-btn.preset { padding: 2px 6px; }

  .stretch-group {
    display: flex;
    align-items: center;
    gap: 3px;
  }

  /* ── Table toolbar ───────────────────────────────────────────────────────── */

  .table-toolbar {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 5px 8px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .search-input {
    flex: 1;
    min-width: 0;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 3px 8px;
    font-size: 0.8rem;
    color: var(--text-primary);
    outline: none;
  }
  .search-input:focus { border-color: var(--accent); }
  .search-input::placeholder { color: var(--text-dim); }

  .filter-select {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 0.78rem;
    color: var(--text-secondary);
    cursor: pointer;
    outline: none;
    max-width: 90px;
  }
  .filter-select:focus { border-color: var(--accent); }

  /* ── Column selector dropdown ─────────────────────────────────────────────── */

  .column-selector {
    position: relative;
    flex-shrink: 0;
  }

  .column-menu {
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 5px 0;
    z-index: 200;
    min-width: 130px;
    box-shadow: 0 6px 16px rgba(0, 0, 0, 0.45);
  }

  .column-menu-item {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 4px 10px;
    font-size: 0.82rem;
    color: var(--text-secondary);
    cursor: pointer;
    user-select: none;
  }
  .column-menu-item:hover { background: var(--bg-row-hover); color: var(--text-primary); }
  .column-menu-item input { accent-color: var(--accent); cursor: pointer; }

  /* ── Two-pane content area ───────────────────────────────────────────────── */

  .content-area {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  .file-list-pane {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: flex 0.18s ease;
    min-width: 0;
  }

  /* ── Table scroll wrapper ────────────────────────────────────────────────── */

  .table-scroll-wrapper {
    flex: 1;
    overflow: auto;
  }

  .table-scroll-wrapper::-webkit-scrollbar { width: 6px; height: 6px; }
  .table-scroll-wrapper::-webkit-scrollbar-track { background: var(--bg-base); }
  .table-scroll-wrapper::-webkit-scrollbar-thumb { background: var(--border-accent); border-radius: 3px; }

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
    border-collapse: collapse;
    font-size: 0.875rem;
    table-layout: fixed;
  }

  .file-table thead tr {
    background: var(--bg-panel);
    position: sticky;
    top: 0;
    z-index: 1;
  }

  .file-table th {
    padding: 7px 10px 7px 8px;
    text-align: left;
    font-size: 0.72rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-dim);
    border-bottom: 1px solid var(--border);
    position: relative;
    overflow: hidden;
    white-space: nowrap;
    user-select: none;
  }

  .file-table th.drag-over { border-left: 2px solid var(--accent); }

  .th-text {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    padding-right: 6px;
  }

  /* Column resize handle — right edge of each th */
  .resize-handle {
    position: absolute;
    right: 0;
    top: 0;
    bottom: 0;
    width: 5px;
    cursor: col-resize;
    background: transparent;
    transition: background 0.1s;
  }
  .resize-handle:hover { background: var(--accent); opacity: 0.5; }

  .file-row td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .file-row:hover td { background: var(--bg-row-hover); }
  .file-row.is-dir  { cursor: pointer; }
  .file-row.is-fits { cursor: pointer; }

  .file-row.is-dir td { background: var(--bg-row-dir); color: var(--accent); }
  .file-row.is-dir:hover td,
  .file-row.is-fits:hover td { background: var(--bg-row-hover); }

  .file-row.selected td { background: var(--accent-dim) !important; color: var(--text-primary); }

  /* Per-column text alignment */
  .col-size, .col-expTime, .col-gain, .col-ccdTemp {
    text-align: right;
    color: var(--text-secondary);
    font-size: 0.82rem;
    font-variant-numeric: tabular-nums;
  }
  .col-modTime, .col-dateObs {
    color: var(--text-secondary);
    font-size: 0.82rem;
    font-variant-numeric: tabular-nums;
  }
  .col-filter, .col-object { font-size: 0.83rem; }

  .file-icon { margin-right: 6px; font-size: 0.9em; }

  /* ── Group header row ─────────────────────────────────────────────────────── */

  .group-header-row td {
    background: color-mix(in srgb, var(--bg-panel) 85%, var(--accent) 15%);
    border-top: 1px solid var(--border-accent);
    border-bottom: 1px solid var(--border-accent);
    padding: 4px 10px;
  }

  .group-object {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--accent);
  }

  .group-date {
    font-size: 0.74rem;
    color: var(--text-secondary);
    margin-left: 8px;
    font-variant-numeric: tabular-nums;
  }

  .group-count {
    font-size: 0.72rem;
    color: var(--text-dim);
    margin-left: 8px;
  }

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

  /* ── Image viewport ──────────────────────────────────────────────────────── */

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

  /* ── FITS metadata ───────────────────────────────────────────────────────── */

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

  .unindexed-hint {
    color: var(--text-dim);
    opacity: 0.7;
    cursor: default;
  }

  .index-done-hint {
    color: var(--success);
    opacity: 0.8;
  }

  /* ── Build Index button ───────────────────────────────────────────────────── */

  .btn-secondary {
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 5px 13px;
    font-size: 0.82rem;
    cursor: pointer;
    transition: color 0.15s, border-color 0.15s;
    margin-right: 6px;
  }
  .btn-secondary:hover:not(:disabled) { color: var(--accent); border-color: var(--accent); }
  .btn-secondary:disabled { opacity: 0.45; cursor: default; }

  /* ── Index progress bar ──────────────────────────────────────────────────── */

  .index-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 14px;
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
    border-bottom: 1px solid var(--border-accent);
    flex-shrink: 0;
    font-size: 0.78rem;
  }

  .index-phase {
    color: var(--text-primary);
    white-space: nowrap;
    min-width: 120px;
  }

  .index-new {
    color: var(--success);
    margin-left: 4px;
    font-size: 0.74rem;
  }

  .index-track {
    flex: 1;
    height: 4px;
    background: var(--border);
    border-radius: 2px;
    overflow: hidden;
    min-width: 60px;
  }

  .index-fill {
    height: 100%;
    background: var(--accent);
    border-radius: 2px;
    transition: width 0.15s linear;
  }

  .index-file {
    color: var(--text-dim);
    font-family: 'Consolas', 'Fira Code', monospace;
    font-size: 0.72rem;
    max-width: 240px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
