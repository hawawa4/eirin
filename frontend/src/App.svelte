<script lang="ts">
  import { onMount } from "svelte";
  import {
    SelectRootFolder,
    ListDirectoryEnriched,
    BuildIndex,
    CancelIndex,
    RejectFile,
    UnrejectFile,
    HardDeleteFile,
    LoadPrefs,
    SetPref,
  } from "../wailsjs/go/app/App.js";
  import { EventsOn } from "../wailsjs/runtime/runtime.js";
  import type { browser } from "../wailsjs/go/models";
  import {
    DEFAULT_COLUMNS,
    type ColumnDef,
    type CtxMenuState,
    type IndexProgress,
  } from "./lib/types";
  import { isFits } from "./lib/utils";
  import AppHeader from "./components/AppHeader.svelte";
  import NavToolbar from "./components/NavToolbar.svelte";
  import IndexProgressBar from "./components/IndexProgressBar.svelte";
  import FileList from "./components/FileList.svelte";
  import PreviewPane from "./components/PreviewPane.svelte";
  import ContextMenu from "./components/ContextMenu.svelte";
  import HardDeleteModal from "./components/HardDeleteModal.svelte";
  import StatusFooter from "./components/StatusFooter.svelte";

  const PREF_ROOT_FOLDER = "root_folder";
  const PREF_BASIC_COLLAPSED = "basic_collapsed";
  const PREF_ADVANCED_COLLAPSED = "advanced_collapsed";
  const PREF_STRETCH_ENABLED = "stretch_enabled";
  const PREF_STRETCH_LEVEL = "stretch_level";
  const PREF_COLUMN_CONFIG = "column_config";

  // ── File browser state ────────────────────────────────────────────────────
  let rootFolder = $state("");
  let currentPath = $state("");
  let pathHistory = $state<string[]>([]);
  let files = $state<browser.EnrichedFileEntry[]>([]);
  let error = $state("");
  let loading = $state(false);

  // ── Column config ─────────────────────────────────────────────────────────
  let columns = $state<ColumnDef[]>(DEFAULT_COLUMNS.map((c) => ({ ...c })));

  // ── Index builder state ───────────────────────────────────────────────────
  let indexProgress = $state<IndexProgress | null>(null);
  let indexRunning = $derived(
    indexProgress !== null && indexProgress.phase !== "done" && indexProgress.phase !== "cancelled",
  );

  // ── Context menu / delete modal ───────────────────────────────────────────
  let ctxMenu = $state<CtxMenuState | null>(null);
  let confirmDel = $state<{ path: string; name: string } | null>(null);

  // ── Selected file / preview ───────────────────────────────────────────────
  let selectedEntry = $state<browser.EnrichedFileEntry | null>(null);

  // ── Stretch / section collapse (persisted) ────────────────────────────────
  let stretchEnabled = $state(true);
  let stretchLevel = $state(2);
  let basicCollapsed = $state(false);
  let advancedCollapsed = $state(true);

  // ── Pref persistence guard ────────────────────────────────────────────────
  let prefsLoaded = $state(false);

  // ── Pane layout ───────────────────────────────────────────────────────────
  let leftPct = $state(42);
  let collapsed = $state(false);

  // ── Footer counts (updated by FileList) ──────────────────────────────────
  let filteredCount = $state(0);
  let totalCount = $derived(files.length);
  let uncachedCount = $derived(
    files.filter((f) => !f.isDir && isFits(f.name) && !f.hasMeta).length,
  );

  onMount(async () => {
    const p = await LoadPrefs();
    stretchEnabled = p.stretchEnabled;
    stretchLevel = p.stretchLevel;
    basicCollapsed = p.basicCollapsed;
    advancedCollapsed = p.advancedCollapsed;

    if (p.columnConfig) {
      try {
        const saved = JSON.parse(p.columnConfig) as ColumnDef[];
        columns = DEFAULT_COLUMNS.map((def) => {
          const s = saved.find((x) => x.id === def.id);
          return s ? { ...def, ...s } : { ...def };
        });
      } catch {
        /* keep defaults */
      }
    }

    if (p.rootFolder) {
      rootFolder = p.rootFolder;
      await loadDirectory(p.rootFolder);
    }
    prefsLoaded = true;

    EventsOn("index:progress", (data: IndexProgress) => {
      indexProgress = data;
      if ((data.phase === "done" || data.phase === "cancelled") && currentPath) {
        setTimeout(() => loadDirectory(currentPath), 400);
      }
    });
  });

  $effect(() => {
    if (prefsLoaded) SetPref(PREF_STRETCH_ENABLED, String(stretchEnabled));
  });
  $effect(() => {
    if (prefsLoaded) SetPref(PREF_STRETCH_LEVEL, String(stretchLevel));
  });
  $effect(() => {
    if (prefsLoaded) SetPref(PREF_BASIC_COLLAPSED, String(basicCollapsed));
  });
  $effect(() => {
    if (prefsLoaded) SetPref(PREF_ADVANCED_COLLAPSED, String(advancedCollapsed));
  });

  function saveColumnConfig() {
    if (!prefsLoaded) return;
    SetPref(PREF_COLUMN_CONFIG, JSON.stringify(columns));
  }

  // ── Index ─────────────────────────────────────────────────────────────────
  async function startBuildIndex() {
    if (!rootFolder || indexRunning) return;
    indexProgress = {
      phase: "scanning",
      total: 0,
      done: 0,
      indexed: 0,
      errors: 0,
      current: "Starting…",
    };
    await BuildIndex(rootFolder);
  }

  // ── Navigation ────────────────────────────────────────────────────────────
  async function selectFolder() {
    const path = await SelectRootFolder();
    if (path) {
      rootFolder = path;
      if (prefsLoaded) SetPref(PREF_ROOT_FOLDER, path);
      pathHistory = [];
      clearPreview();
      await loadDirectory(path);
    }
  }

  async function loadDirectory(path: string) {
    loading = true;
    error = "";
    try {
      const result = await ListDirectoryEnriched(path);
      files = (result || []).slice().sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      currentPath = path;
    } catch (e) {
      error = String(e);
      files = [];
    } finally {
      loading = false;
    }
  }

  async function navigateBack() {
    if (pathHistory.length === 0) return;
    const prev = pathHistory[pathHistory.length - 1];
    pathHistory = pathHistory.slice(0, -1);
    clearPreview();
    await loadDirectory(prev);
  }

  async function onRowClick(entry: browser.EnrichedFileEntry) {
    if (entry.isDir) {
      clearPreview();
      pathHistory = [...pathHistory, currentPath];
      await loadDirectory(entry.path);
    } else if (isFits(entry.name)) {
      selectedEntry = entry;
    }
  }

  function clearPreview() {
    selectedEntry = null;
  }

  // ── File operations ───────────────────────────────────────────────────────
  async function rejectFile(entry: browser.EnrichedFileEntry) {
    ctxMenu = null;
    await RejectFile(entry.path);
    entry.isRejected = true;
  }

  async function restoreFile(entry: browser.EnrichedFileEntry) {
    ctxMenu = null;
    await UnrejectFile(entry.path);
    entry.isRejected = false;
  }

  function openHardDeleteConfirm(entry: browser.EnrichedFileEntry) {
    ctxMenu = null;
    confirmDel = { path: entry.path, name: entry.name };
  }

  async function doHardDelete() {
    if (!confirmDel) return;
    const { path } = confirmDel;
    confirmDel = null;
    await HardDeleteFile(path);
    files = files.filter((f) => f.path !== path);
    if (selectedEntry?.path === path) clearPreview();
  }

  // ── Pane resize ───────────────────────────────────────────────────────────
  function onDividerMouseDown(e: MouseEvent) {
    e.preventDefault();
    const contentArea = document.querySelector(".content-area") as HTMLElement;

    function onMove(ev: MouseEvent) {
      const rect = contentArea.getBoundingClientRect();
      const pct = ((ev.clientX - rect.left) / rect.width) * 100;
      leftPct = Math.max(15, Math.min(75, pct));
      if (collapsed) collapsed = false;
    }
    function onUp() {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    }
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  let leftStyle = $derived(
    selectedEntry
      ? collapsed
        ? "flex: 0 0 0px; min-width: 0; overflow: hidden;"
        : `flex: 0 0 ${leftPct}%;`
      : "flex: 1;",
  );
</script>

<div class="layout">
  <AppHeader
    {rootFolder}
    {indexRunning}
    onselectfolder={selectFolder}
    onbuildindex={startBuildIndex}
  />

  {#if !rootFolder}
    <div class="empty-state">
      <div class="empty-icon">◎</div>
      <p class="empty-title">No folder selected</p>
      <p class="empty-sub">Choose your astrophotography NAS folder to get started</p>
      <button class="btn-primary btn-large" onclick={selectFolder}>Select Root Folder</button>
    </div>
  {:else}
    <NavToolbar {currentPath} canGoBack={pathHistory.length > 0} onnavigateBack={navigateBack} />

    {#if indexProgress && indexProgress.phase !== "done" && indexProgress.phase !== "cancelled"}
      <IndexProgressBar progress={indexProgress} oncancel={() => CancelIndex()} />
    {/if}

    <div class="content-area">
      <div class="file-list-pane" style={leftStyle}>
        <FileList
          {files}
          {selectedEntry}
          {columns}
          {error}
          {loading}
          onfileclick={onRowClick}
          oncontextmenu={(x, y, entry) => {
            ctxMenu = { x, y, entry };
          }}
          onsavecolumns={saveColumnConfig}
          onfilteredcountchange={(n) => {
            filteredCount = n;
          }}
        />
      </div>

      {#if selectedEntry}
        <div class="divider" onmousedown={onDividerMouseDown}>
          <button
            class="collapse-btn"
            onmousedown={(e) => e.stopPropagation()}
            onclick={() => (collapsed = !collapsed)}
            title={collapsed ? "Expand file list" : "Collapse file list"}
            >{collapsed ? "›" : "‹"}</button
          >
        </div>

        <PreviewPane
          entry={selectedEntry}
          bind:stretchEnabled
          bind:stretchLevel
          bind:basicCollapsed
          bind:advancedCollapsed
          onclose={clearPreview}
        />
      {/if}
    </div>

    <StatusFooter
      {totalCount}
      {filteredCount}
      {uncachedCount}
      {indexRunning}
      {indexProgress}
      {rootFolder}
    />
  {/if}
</div>

{#if ctxMenu}
  <ContextMenu
    menu={ctxMenu}
    onclose={() => {
      ctxMenu = null;
    }}
    onreject={rejectFile}
    onrestore={restoreFile}
    onharddelete={openHardDeleteConfirm}
  />
{/if}

{#if confirmDel}
  <HardDeleteModal
    target={confirmDel}
    onconfirm={doHardDelete}
    oncancel={() => {
      confirmDel = null;
    }}
  />
{/if}

<style>
  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

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

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
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

  /* ── Resize divider ─────────────────────────────────────────────────────── */

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

  .divider:hover {
    background: var(--accent-dim);
  }

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
    transition:
      color 0.15s,
      border-color 0.15s;
  }
  .collapse-btn:hover {
    color: var(--accent);
    border-color: var(--accent);
  }
</style>
