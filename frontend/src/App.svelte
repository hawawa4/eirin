<script lang="ts">
  import { onMount, tick } from "svelte";
  import { SelectRootFolder, GetAppInfo, CheckSiril, GetProjectsFolder } from "$app";
  import {
    DEFAULT_COLUMNS,
    DEFAULT_LIBRARY_COLUMNS,
    type AppInfo,
    type AppMode,
    type ColumnDef,
    type Project,
    type SirilInfo,
    type Theme,
  } from "./lib/types";
  import AppHeader from "./components/AppHeader.svelte";
  import IndexProgressBar from "./components/IndexProgressBar.svelte";
  import LibraryView from "./components/LibraryView.svelte";
  import ViewerLibrary from "./components/viewer/ViewerLibrary.svelte";
  import ImportView from "./components/ImportView.svelte";
  import ProjectsView from "./components/ProjectsView.svelte";
  import SettingsView from "./components/SettingsView.svelte";
  import StorageView from "./components/StorageView.svelte";
  import SkyAtlas from "./components/SkyAtlas.svelte";
  import CoverageView from "./components/CoverageView.svelte";
  import BrowseView from "./components/browse/BrowseView.svelte";
  import Toaster from "./components/Toaster.svelte";
  import { loadUiState } from "./lib/uiState.svelte";
  import { installUiScaleShortcuts } from "./lib/uiScale.svelte";
  import { loadPreviewPrefs } from "./lib/previewPrefs.svelte";
  import { toast } from "./lib/toast.svelte";
  import { loadPrefs, savePref, usePrefsLocally } from "./lib/prefs";
  import { indexer, type IndexOutcome, type StartOptions } from "./lib/shell/indexing.svelte";
  import { DEFAULT_MODE, MODES, visibleModes } from "./lib/shell/modes";

  const PREF_ROOT_FOLDER = "root_folder";
  const PREF_COLUMN_CONFIG = "column_config";
  const PREF_LIBRARY_COLUMN_CONFIG = "library_column_config";
  const PREF_THEME = "theme";

  // ── App mode ──────────────────────────────────────────────────────────────
  let appMode = $state<AppMode>(DEFAULT_MODE);
  /** Views stay mounted (hidden) once visited, so they keep their state across tabs. */
  let visited = $state<Partial<Record<AppMode, boolean>>>({ [DEFAULT_MODE]: true });
  let modeRequiresRoot = $derived(MODES.find((m) => m.value === appMode)?.requiresRoot ?? true);

  function setMode(m: AppMode) {
    appMode = m;
    visited[m] = true;
  }

  // ── Theme ─────────────────────────────────────────────────────────────────
  let theme = $state<Theme>("blue");

  // ── Root folder / column configs ──────────────────────────────────────────
  let rootFolder = $state("");
  let columns = $state<ColumnDef[]>(DEFAULT_COLUMNS.map((c) => ({ ...c })));
  let libraryColumns = $state<ColumnDef[]>(DEFAULT_LIBRARY_COLUMNS.map((c) => ({ ...c })));

  // ── App info (DB path, server URL) ────────────────────────────────────────
  // Starts out read-only so a server visitor never sees desktop controls
  // before GetAppInfo answers; views only mount once the prefs have loaded.
  let appInfo = $state<AppInfo>({
    dbPath: "",
    serverPort: 7070,
    serverUrl: "",
    portSource: "",
    capabilities: { desktopMode: false, readOnly: true },
  });
  let desktopMode = $derived(appInfo.capabilities.desktopMode);
  let readOnly = $derived(appInfo.capabilities.readOnly);

  // ── Siril ─────────────────────────────────────────────────────────────────
  let sirilInfo = $state<SirilInfo>({ executable: "siril", version: "…", available: false });
  let sirilAvailable = $derived(sirilInfo.available && desktopMode);

  // ── Projects ──────────────────────────────────────────────────────────────
  let projectsFolder = $state("");

  // ── Pref persistence guard ────────────────────────────────────────────────
  let prefsLoaded = $state(false);

  // ── View refs ─────────────────────────────────────────────────────────────
  let libraryView = $state<{
    reload: () => void;
    focusFile: (nasPath: string) => void;
    selectFiles?: (paths: string[]) => void;
  } | null>(null);
  let projectsView = $state<{ selectProjectById: (id: number) => void } | null>(null);
  let browseView = $state<{ reload: () => void } | null>(null);
  let skyAtlas = $state<{ focusFrame: (nasPath: string) => void } | null>(null);

  // ── Read-only viewer guard — only the viewer's tabs ───────────────────────
  $effect(() => {
    if (readOnly && !visibleModes(true).some((m) => m.value === appMode)) setMode("library");
  });

  onMount(() => installUiScaleShortcuts());

  // Imports index each copied file themselves (and emit library:updated), so
  // they don't trigger a scan here.
  onMount(() => indexer.listen(onIndexFinished));

  onMount(async () => {
    let p;
    try {
      const info = await GetAppInfo();
      appInfo = info;
      usePrefsLocally(info.capabilities.readOnly);
      p = await loadPrefs();
    } catch (e) {
      toast.error(`Couldn't load settings: ${String(e)}`);
      return;
    }
    if (desktopMode) {
      const [siril, pf] = await Promise.allSettled([CheckSiril(), GetProjectsFolder()]);
      if (siril.status === "fulfilled") sirilInfo = siril.value;
      if (pf.status === "fulfilled") projectsFolder = pf.value;
    }

    loadPreviewPrefs(p);
    if (p.theme === "red" || p.theme === "grey") theme = p.theme;
    loadUiState(p.uiState);
    columns = mergeColumns(DEFAULT_COLUMNS, p.columnConfig);
    libraryColumns = mergeColumns(DEFAULT_LIBRARY_COLUMNS, p.libraryColumnConfig);
    if (p.rootFolder) rootFolder = p.rootFolder;
    prefsLoaded = true;
  });

  $effect(() => {
    if (theme === "blue") {
      document.documentElement.removeAttribute("data-theme");
    } else {
      document.documentElement.setAttribute("data-theme", theme);
    }
    if (prefsLoaded) savePref(PREF_THEME, theme);
  });

  function mergeColumns(defaults: ColumnDef[], json: string | undefined): ColumnDef[] {
    if (json) {
      try {
        const saved = JSON.parse(json) as ColumnDef[];
        return defaults.map((def) => {
          const s = saved.find((x) => x.id === def.id);
          return s ? { ...def, ...s } : { ...def };
        });
      } catch {
        /* keep defaults */
      }
    }
    return defaults.map((c) => ({ ...c }));
  }

  function saveColumnConfig() {
    if (prefsLoaded) savePref(PREF_COLUMN_CONFIG, JSON.stringify(columns));
  }

  function saveLibraryColumnConfig() {
    if (prefsLoaded) savePref(PREF_LIBRARY_COLUMN_CONFIG, JSON.stringify(libraryColumns));
  }

  // ── Library scan (index) ──────────────────────────────────────────────────
  function startScan(opts: StartOptions = {}) {
    void indexer.start(rootFolder, opts);
  }

  function onIndexFinished(o: IndexOutcome) {
    if (o.kind === "error") {
      toast.error(`Library scan failed: ${o.message}`);
    } else if (o.kind === "cancelled") {
      toast.info("Library scan cancelled");
    } else {
      const { indexed, errors } = o.progress;
      let msg = `Library scan complete — ${indexed} new/updated frame${indexed !== 1 ? "s" : ""}`;
      if (errors > 0) msg += ` · ${errors} file${errors !== 1 ? "s" : ""} couldn't be read`;
      const offerLibrary = indexed > 0 && appMode !== "library";
      toast.success(msg, {
        action: offerLibrary ? { label: "View library", run: () => setMode("library") } : undefined,
      });
    }
    if (o.kind !== "error" || o.progress) {
      setTimeout(() => {
        browseView?.reload();
        libraryView?.reload();
      }, 400);
    }
  }

  // ── Root folder ───────────────────────────────────────────────────────────
  async function selectFolder() {
    let path: string;
    try {
      path = await SelectRootFolder();
    } catch (e) {
      toast.error(`Couldn't select the folder: ${String(e)}`);
      return;
    }
    if (!path || path === rootFolder) return;
    rootFolder = path;
    if (prefsLoaded) savePref(PREF_ROOT_FOLDER, path);
    // Pick up whatever is already in the new root; supersedes a scan of the old one.
    startScan({ ifRunning: "supersede" });
  }

  // ── Cross-view navigation ─────────────────────────────────────────────────
  async function openInLibrary(nasPath: string) {
    setMode("library");
    await tick();
    libraryView?.focusFile(nasPath);
  }

  async function selectInLibrary(paths: string[]) {
    setMode("library");
    await tick();
    libraryView?.selectFiles?.(paths);
  }

  async function showOnAtlas(nasPath: string) {
    setMode("atlas");
    await tick();
    skyAtlas?.focusFrame(nasPath);
  }

  async function openProject(project: Project) {
    setMode("projects");
    await tick();
    projectsView?.selectProjectById(project.id);
  }
</script>

<div class="layout">
  <AppHeader
    {rootFolder}
    {appMode}
    {theme}
    {readOnly}
    onmodechange={setMode}
    onthemechange={(t) => {
      theme = t;
    }}
  />

  <!-- Global index progress bar — visible in all modes while scanning -->
  {#if indexer.running && indexer.progress}
    <IndexProgressBar progress={indexer.progress} oncancel={() => indexer.cancel()} />
  {/if}

  <main class="views">
    {#if !rootFolder && modeRequiresRoot}
      <div class="empty-state">
        <div class="empty-icon" aria-hidden="true">◎</div>
        <p class="empty-title">No folder selected</p>
        {#if desktopMode}
          <p class="empty-sub">Choose your astrophotography NAS folder to get started</p>
          <button class="btn-primary btn-large" onclick={selectFolder}>Select Root Folder</button>
        {:else}
          <p class="empty-sub">
            No library folder is configured on this server. Set <code>EIRIN_ROOT</code> to where the library
            is mounted.
          </p>
        {/if}
      </div>
    {/if}

    {#if rootFolder}
      <!-- Re-key on root change so every view starts fresh against the new library. -->
      {#key rootFolder}
        {#if visited.library}
          <div class="view" class:hidden={appMode !== "library"}>
            {#if readOnly}
              <ViewerLibrary
                bind:this={libraryView}
                {rootFolder}
                active={appMode === "library"}
                onshowonatlas={showOnAtlas}
              />
            {:else}
              <LibraryView
                bind:this={libraryView}
                {rootFolder}
                columns={libraryColumns}
                {sirilAvailable}
                active={appMode === "library"}
                indexRunning={indexer.running}
                onscan={() => startScan()}
                onsavecolumns={saveLibraryColumnConfig}
                oncreateproject={openProject}
              />
            {/if}
          </div>
        {/if}

        {#if desktopMode && visited.projects}
          <div class="view" class:hidden={appMode !== "projects"}>
            <ProjectsView
              bind:this={projectsView}
              {rootFolder}
              {projectsFolder}
              {sirilAvailable}
              active={appMode === "projects"}
              onscan={() => startScan()}
              onopeninlibrary={openInLibrary}
            />
          </div>
        {/if}

        {#if desktopMode && visited.import}
          <div class="view" class:hidden={appMode !== "import"}>
            <ImportView
              {rootFolder}
              active={appMode === "import"}
              onviewlibrary={() => setMode("library")}
            />
          </div>
        {/if}

        {#if visited.atlas}
          <div class="view" class:hidden={appMode !== "atlas"}>
            <SkyAtlas
              bind:this={skyAtlas}
              rootPath={rootFolder}
              active={appMode === "atlas"}
              {theme}
              {readOnly}
              onscan={readOnly ? undefined : () => startScan()}
              onframeopen={openInLibrary}
            />
          </div>
        {/if}

        {#if desktopMode && visited.coverage}
          <div class="view" class:hidden={appMode !== "coverage"}>
            <CoverageView
              rootPath={rootFolder}
              active={appMode === "coverage"}
              {theme}
              onscan={() => startScan()}
              onopenproject={openProject}
              onshowinlibrary={selectInLibrary}
            />
          </div>
        {/if}

        {#if !readOnly && visited.storage}
          <div class="view" class:hidden={appMode !== "storage"}>
            <StorageView
              rootPath={rootFolder}
              active={appMode === "storage"}
              onscan={() => startScan()}
            />
          </div>
        {/if}

        {#if !readOnly && visited.browser}
          <div class="view" class:hidden={appMode !== "browser"}>
            <BrowseView
              bind:this={browseView}
              {rootFolder}
              {columns}
              {sirilAvailable}
              {desktopMode}
              active={appMode === "browser"}
              indexRunning={indexer.running}
              indexProgress={indexer.progress}
              onsavecolumns={saveColumnConfig}
              onscan={() => startScan()}
            />
          </div>
        {/if}
      {/key}
    {/if}

    {#if appMode === "settings"}
      <div class="view">
        <SettingsView
          {rootFolder}
          {projectsFolder}
          {appInfo}
          indexRunning={indexer.running}
          onselectfolder={selectFolder}
          onbuildindex={() => startScan()}
          onrebuildindex={() => startScan({ force: true })}
          onsirilchange={(info) => (sirilInfo = info)}
          onprojectsfolderset={(path) => {
            projectsFolder = path;
          }}
        />
      </div>
    {/if}
  </main>
</div>

<Toaster />

<style>
  .layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  .views {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
  }

  .view {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
    overflow: hidden;
  }
  .view.hidden {
    display: none;
  }

  .empty-state {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 24px;
  }

  .empty-icon {
    font-size: 3rem;
    line-height: 1;
    color: var(--accent);
    margin-bottom: 8px;
  }
  .empty-title {
    font-size: var(--fs-xl);
    font-weight: 500;
    color: var(--text-primary);
  }
  .empty-sub {
    font-size: var(--fs-md);
    color: var(--text-secondary);
    max-width: 380px;
    text-align: center;
  }
</style>
