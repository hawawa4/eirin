<script lang="ts">
  import { onMount } from "svelte";
  import {
    ListProjects,
    CreateProject,
    DeleteProject,
    GetProjectFrames,
    AddFramesToProject,
    OpenProjectInSiril,
    GetLightObjects,
    GetLightFramesPaged,
    GetProjectOutputFiles,
    ImportOutputFiles,
  } from "../../wailsjs/go/app/App.js";
  import type { app } from "../../wailsjs/go/models";
  import type { Project, ProjectOutputFile } from "../lib/types";

  interface Props {
    rootFolder: string;
    projectsFolder: string;
  }

  let { rootFolder, projectsFolder }: Props = $props();

  // ── Project list ──────────────────────────────────────────────────────────
  let projects = $state<Project[]>([]);
  let selected = $state<Project | null>(null);
  let loadingProjects = $state(false);

  // ── Lights (collapsible) ──────────────────────────────────────────────────
  let lightsCollapsed = $state(false);
  let projectFrames = $state<string[]>([]);
  let loadingFrames = $state(false);

  // ── Output files (polled) ─────────────────────────────────────────────────
  let outputFiles = $state<ProjectOutputFile[]>([]);

  // Poll the project root for new output files while a project is selected.
  // The poll runs every 3 s; the effect re-starts whenever `selected` changes.
  $effect(() => {
    const proj = selected;
    if (!proj) {
      outputFiles = [];
      return;
    }
    let cancelled = false;
    const poll = async () => {
      if (cancelled) return;
      try {
        const files = await GetProjectOutputFiles(proj.folder);
        if (!cancelled) outputFiles = files;
      } catch {
        /* ignore transient errors */
      }
    };
    poll();
    const id = setInterval(poll, 3000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  });

  // ── Create form ───────────────────────────────────────────────────────────
  let showCreate = $state(false);
  let createName = $state("");
  let createDesc = $state("");
  let creating = $state(false);
  let createError = $state("");

  // ── Frame picker (Add lights) ─────────────────────────────────────────────
  let showPicker = $state(false);
  let pickerStep = $state<"objects" | "frames">("objects");
  let availableObjects = $state<string[]>([]);
  let pickerObjects = $state<Set<string>>(new Set());
  let pickerFrames = $state<app.LibraryFrame[]>([]);
  let pickerHasMore = $state(false);
  let pickerOffset = $state(0);
  let pickerLoading = $state(false);
  let pickerLoadingMore = $state(false);
  let pickerFilter = $state("");
  let pickerSelected = $state<Set<string>>(new Set());
  let addMode = $state<"symlink" | "copy">("symlink");
  let adding = $state(false);
  let addError = $state("");

  // ── Import outputs to NAS ─────────────────────────────────────────────────
  let showImport = $state(false);
  let importSelected = $state<Set<string>>(new Set());
  let importSubfolder = $state("");
  let importing = $state(false);
  let importError = $state("");

  // ── Delete confirm ────────────────────────────────────────────────────────
  let confirmDelete = $state<Project | null>(null);

  onMount(async () => {
    await loadProjects();
  });

  async function loadProjects() {
    loadingProjects = true;
    try {
      projects = await ListProjects();
    } finally {
      loadingProjects = false;
    }
  }

  async function selectProject(p: Project) {
    selected = p;
    importSubfolder = p.name;
    await reloadFrames();
  }

  async function reloadFrames() {
    if (!selected) return;
    loadingFrames = true;
    try {
      projectFrames = await GetProjectFrames(selected.folder);
    } finally {
      loadingFrames = false;
    }
  }

  async function doCreate() {
    if (!createName.trim()) return;
    creating = true;
    createError = "";
    try {
      const p = await CreateProject(createName.trim(), createDesc.trim());
      projects = [p, ...projects];
      showCreate = false;
      createName = "";
      createDesc = "";
      await selectProject(p);
    } catch (e) {
      createError = String(e);
    } finally {
      creating = false;
    }
  }

  async function doDelete(p: Project) {
    confirmDelete = null;
    await DeleteProject(p.id);
    projects = projects.filter((x) => x.id !== p.id);
    if (selected?.id === p.id) {
      selected = null;
      projectFrames = [];
    }
  }

  // ── Lights picker ─────────────────────────────────────────────────────────
  async function openPicker() {
    pickerStep = "objects";
    pickerFilter = "";
    pickerSelected = new Set();
    pickerObjects = new Set();
    pickerFrames = [];
    pickerHasMore = false;
    pickerOffset = 0;
    addError = "";
    showPicker = true;
    if (!rootFolder) return;
    pickerLoading = true;
    try {
      availableObjects = (await GetLightObjects(rootFolder)) ?? [];
    } finally {
      pickerLoading = false;
    }
  }

  async function loadPickerFrames(reset: boolean) {
    if (reset) {
      pickerFrames = [];
      pickerOffset = 0;
      pickerHasMore = false;
    }
    pickerLoadingMore = true;
    try {
      const result = await GetLightFramesPaged(rootFolder, [...pickerObjects], pickerOffset);
      pickerFrames = reset ? (result.frames ?? []) : [...pickerFrames, ...(result.frames ?? [])];
      pickerHasMore = result.hasMore;
      pickerOffset = pickerFrames.length;
    } finally {
      pickerLoadingMore = false;
    }
  }

  async function goToFrameStep() {
    pickerStep = "frames";
    pickerSelected = new Set();
    pickerFilter = "";
    await loadPickerFrames(true);
  }

  let filteredLights = $derived(
    pickerFrames.filter(
      (f) =>
        !pickerFilter ||
        f.fileName.toLowerCase().includes(pickerFilter.toLowerCase()),
    ),
  );

  function togglePick(nasPath: string) {
    const next = new Set(pickerSelected);
    if (next.has(nasPath)) next.delete(nasPath);
    else next.add(nasPath);
    pickerSelected = next;
  }

  function toggleAllLights() {
    pickerSelected =
      pickerSelected.size === filteredLights.length
        ? new Set()
        : new Set(filteredLights.map((f) => f.nasPath));
  }

  function toggleObject(obj: string) {
    const next = new Set(pickerObjects);
    if (next.has(obj)) next.delete(obj);
    else next.add(obj);
    pickerObjects = next;
  }

  async function doAddFrames() {
    if (!selected || pickerSelected.size === 0) return;
    adding = true;
    addError = "";
    try {
      await AddFramesToProject(selected.folder, [...pickerSelected], addMode);
      showPicker = false;
      await reloadFrames();
    } catch (e) {
      addError = String(e);
    } finally {
      adding = false;
    }
  }

  // ── Import outputs ────────────────────────────────────────────────────────
  function openImport() {
    importSelected = new Set(outputFiles.map((f) => f.path));
    importSubfolder = selected?.name ?? "";
    importError = "";
    showImport = true;
  }

  function toggleImportFile(path: string) {
    const next = new Set(importSelected);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    importSelected = next;
  }

  function toggleAllImport() {
    importSelected =
      importSelected.size === outputFiles.length
        ? new Set()
        : new Set(outputFiles.map((f) => f.path));
  }

  async function doImportOutputs() {
    if (!rootFolder || importSelected.size === 0) return;
    importing = true;
    importError = "";
    try {
      const destFolder = importSubfolder.trim()
        ? `${rootFolder}/${importSubfolder.trim()}`
        : rootFolder;
      await ImportOutputFiles([...importSelected], destFolder);
      showImport = false;
    } catch (e) {
      importError = String(e);
    } finally {
      importing = false;
    }
  }

  // ── Misc ──────────────────────────────────────────────────────────────────
  async function doOpenInSiril() {
    if (!selected) return;
    await OpenProjectInSiril(selected.folder);
  }

  function formatDate(iso: string) {
    try {
      return new Date(iso).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      });
    } catch {
      return iso;
    }
  }

  function formatSize(bytes: number) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
</script>

<div class="projects-layout">
  <!-- ── Left: project list ────────────────────────────────────────────────── -->
  <aside class="project-sidebar">
    <div class="sidebar-header">
      <span class="sidebar-title">Projects</span>
      <button class="btn-icon" onclick={() => (showCreate = !showCreate)} title="New project">+</button>
    </div>

    {#if showCreate}
      <div class="create-form">
        <input
          class="create-input"
          type="text"
          placeholder="Project name"
          bind:value={createName}
          spellcheck="false"
        />
        <input
          class="create-input"
          type="text"
          placeholder="Description (optional)"
          bind:value={createDesc}
          spellcheck="false"
        />
        {#if createError}<p class="form-error">{createError}</p>{/if}
        <div class="create-actions">
          <button class="btn-primary" onclick={doCreate} disabled={creating || !createName.trim()}>
            {creating ? "Creating…" : "Create"}
          </button>
          <button
            class="btn-ghost"
            onclick={() => {
              showCreate = false;
              createName = "";
              createDesc = "";
              createError = "";
            }}>Cancel</button
          >
        </div>
      </div>
    {/if}

    {#if !projectsFolder}
      <p class="sidebar-hint">Configure a Projects folder in Settings first.</p>
    {:else if loadingProjects}
      <p class="sidebar-hint">Loading…</p>
    {:else if projects.length === 0}
      <p class="sidebar-hint">No projects yet. Click + to create one.</p>
    {:else}
      <ul class="project-list">
        {#each projects as p (p.id)}
          <li
            class="project-item"
            class:active={selected?.id === p.id}
            onclick={() => selectProject(p)}
          >
            <span class="project-name">{p.name}</span>
            <span class="project-date">{formatDate(p.createdAt)}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </aside>

  <!-- ── Right: project detail ─────────────────────────────────────────────── -->
  <main class="project-detail">
    {#if !selected}
      <div class="empty-detail">
        <div class="empty-icon">◫</div>
        <p class="empty-title">Select a project</p>
        <p class="empty-sub">Choose a project from the list, or create a new one.</p>
      </div>
    {:else}
      <!-- Header -->
      <div class="detail-header">
        <div class="detail-meta">
          <h2 class="detail-name">{selected.name}</h2>
          {#if selected.description}<p class="detail-desc">{selected.description}</p>{/if}
          <p class="detail-folder" title={selected.folder}>{selected.folder}</p>
        </div>
        <div class="detail-actions">
          <button class="btn-primary" onclick={doOpenInSiril}>Open in Siril</button>
          <button
            class="btn-ghost danger"
            onclick={() => (confirmDelete = selected)}
            title="Remove from list — does not delete files"
          >Remove</button>
        </div>
      </div>

      <div class="detail-body">
        <!-- ── Lights section (collapsible) ─────────────────────────────── -->
        <section class="detail-section">
          <div class="section-hdr-row">
            <button
              class="section-hdr"
              onclick={() => (lightsCollapsed = !lightsCollapsed)}
            >
              <span class="section-chevron">{lightsCollapsed ? "▶" : "▼"}</span>
              <span class="section-title">Lights</span>
              <span class="section-count">{projectFrames.length}</span>
            </button>
            <button
              class="btn-secondary small"
              onclick={() => openPicker()}
            >Add frames…</button>
          </div>

          {#if !lightsCollapsed}
            <div class="section-body">
              {#if loadingFrames}
                <p class="hint">Loading…</p>
              {:else if projectFrames.length === 0}
                <p class="hint">No frames yet. Click "Add frames" to link light frames from your library.</p>
              {:else}
                <div class="file-scroll">
                  <table class="file-table">
                    <thead><tr><th>Filename</th></tr></thead>
                    <tbody>
                      {#each projectFrames as name (name)}
                        <tr><td class="mono-cell">{name}</td></tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            </div>
          {/if}
        </section>

        <!-- ── Project outputs (auto-watched) ───────────────────────────── -->
        <section class="detail-section">
          <div class="section-hdr no-toggle">
            <span class="section-title">Project Outputs</span>
            <span class="section-count">{outputFiles.length}</span>
            <span class="pulse-dot" title="Watching for new files"></span>
            <span class="section-spacer"></span>
            {#if outputFiles.length > 0}
              <button class="btn-secondary small" onclick={openImport}>Import to NAS…</button>
            {/if}
          </div>

          <div class="section-body">
            {#if outputFiles.length === 0}
              <p class="hint">
                No output files yet. Process your lights in Siril and save the results to
                <code>{selected.folder}</code>.
              </p>
            {:else}
              <div class="file-scroll">
                <table class="file-table">
                  <thead>
                    <tr>
                      <th>Filename</th>
                      <th class="col-size">Size</th>
                      <th class="col-date">Modified</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each outputFiles as f (f.path)}
                      <tr>
                        <td class="mono-cell">{f.name}</td>
                        <td class="col-size dim-cell">{formatSize(f.size)}</td>
                        <td class="col-date dim-cell">{f.modTime.slice(0, 10)}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          </div>
        </section>
      </div>
    {/if}
  </main>
</div>

<!-- ── Delete confirm ─────────────────────────────────────────────────────── -->
{#if confirmDelete}
  <div class="modal-backdrop" onclick={() => (confirmDelete = null)}>
    <div class="modal" onclick={(e) => e.stopPropagation()}>
      <p class="modal-title">Remove <strong>{confirmDelete.name}</strong>?</p>
      <p class="modal-sub">The project folder and files will not be deleted from disk.</p>
      <div class="modal-actions">
        <button class="btn-danger" onclick={() => doDelete(confirmDelete!)}>Remove</button>
        <button class="btn-ghost" onclick={() => (confirmDelete = null)}>Cancel</button>
      </div>
    </div>
  </div>
{/if}

<!-- ── Add lights modal ───────────────────────────────────────────────────── -->
{#if showPicker}
  <div class="modal-backdrop" onclick={() => (showPicker = false)}>
    <div class="picker-modal" onclick={(e) => e.stopPropagation()}>
      <div class="picker-header">
        <span class="picker-title">
          {pickerStep === "objects" ? "Select objects" : "Add light frames"} — {selected?.name}
        </span>
        <button class="btn-ghost" onclick={() => (showPicker = false)}>✕</button>
      </div>

      {#if pickerStep === "objects"}
        <!-- Step 1: Pick objects -->
        <div class="picker-list-scroll">
          {#if pickerLoading}
            <p class="hint padded">Loading objects…</p>
          {:else if availableObjects.length === 0}
            <p class="hint padded">No indexed light frames found. Run Build Index first.</p>
          {:else}
            <div class="object-list">
              {#each availableObjects as obj}
                <label class="object-item">
                  <input
                    type="checkbox"
                    checked={pickerObjects.has(obj)}
                    onchange={() => toggleObject(obj)}
                  />
                  <span class="object-name">{obj || "(no object)"}</span>
                </label>
              {/each}
            </div>
          {/if}
        </div>
        <div class="picker-footer">
          <div class="picker-actions">
            <button
              class="btn-primary"
              disabled={pickerObjects.size === 0 || pickerLoading}
              onclick={goToFrameStep}
            >
              Continue ({pickerObjects.size} object{pickerObjects.size !== 1 ? "s" : ""})
            </button>
            <button class="btn-ghost" onclick={() => (showPicker = false)}>Cancel</button>
          </div>
        </div>

      {:else}
        <!-- Step 2: Pick frames (paginated) -->
        <div class="picker-toolbar">
          <button class="btn-ghost small" onclick={() => { pickerStep = "objects"; }}>← Back</button>
          <input
            class="picker-search"
            type="search"
            placeholder="Filter by name…"
            bind:value={pickerFilter}
          />
          <button class="btn-ghost small" onclick={toggleAllLights}>
            {pickerSelected.size === filteredLights.length && filteredLights.length > 0
              ? "Deselect all"
              : "Select all"}
          </button>
          <span class="picker-count">{pickerSelected.size} selected</span>
        </div>

        <div class="picker-list-scroll">
          {#if pickerLoadingMore && pickerFrames.length === 0}
            <p class="hint padded">Loading frames…</p>
          {:else if filteredLights.length === 0}
            <p class="hint padded">No frames match the filter.</p>
          {:else}
            <table class="picker-table">
              <thead>
                <tr>
                  <th class="col-check"></th>
                  <th>Name</th>
                  <th>Object</th>
                  <th>Filter</th>
                  <th>Date</th>
                </tr>
              </thead>
              <tbody>
                {#each filteredLights as f (f.nasPath)}
                  <tr
                    class="picker-row"
                    class:picked={pickerSelected.has(f.nasPath)}
                    onclick={() => togglePick(f.nasPath)}
                  >
                    <td class="col-check">
                      <input
                        type="checkbox"
                        checked={pickerSelected.has(f.nasPath)}
                        onclick={(e) => e.stopPropagation()}
                        onchange={() => togglePick(f.nasPath)}
                      />
                    </td>
                    <td class="col-name">{f.fileName}</td>
                    <td class="dim-cell">{f.object || "—"}</td>
                    <td class="dim-cell">{f.filter || "—"}</td>
                    <td class="dim-cell">{f.dateObs ? f.dateObs.slice(0, 10) : "—"}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
            {#if pickerHasMore}
              <div class="load-more-row">
                <button
                  class="btn-ghost small"
                  disabled={pickerLoadingMore}
                  onclick={() => loadPickerFrames(false)}
                >
                  {pickerLoadingMore ? "Loading…" : "Load more"}
                </button>
              </div>
            {/if}
          {/if}
        </div>

        <div class="picker-footer">
          <div class="mode-toggle">
            <label class="mode-opt"><input type="radio" name="addMode" value="symlink" bind:group={addMode} /> Symlink</label>
            <label class="mode-opt"><input type="radio" name="addMode" value="copy" bind:group={addMode} /> Copy</label>
          </div>
          {#if addError}<p class="form-error">{addError}</p>{/if}
          <div class="picker-actions">
            <button class="btn-primary" onclick={doAddFrames} disabled={adding || pickerSelected.size === 0}>
              {adding ? "Adding…" : `Add ${pickerSelected.size} frame${pickerSelected.size !== 1 ? "s" : ""}`}
            </button>
            <button class="btn-ghost" onclick={() => (showPicker = false)}>Cancel</button>
          </div>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- ── Import outputs modal ───────────────────────────────────────────────── -->
{#if showImport}
  <div class="modal-backdrop" onclick={() => (showImport = false)}>
    <div class="picker-modal" onclick={(e) => e.stopPropagation()}>
      <div class="picker-header">
        <span class="picker-title">Import outputs to NAS</span>
        <button class="btn-ghost" onclick={() => (showImport = false)}>✕</button>
      </div>

      <div class="picker-toolbar">
        <button class="btn-ghost small" onclick={toggleAllImport}>
          {importSelected.size === outputFiles.length && outputFiles.length > 0
            ? "Deselect all"
            : "Select all"}
        </button>
        <span class="picker-count">{importSelected.size} selected</span>
      </div>

      <div class="picker-list-scroll">
        <table class="picker-table">
          <thead>
            <tr>
              <th class="col-check"></th>
              <th>Filename</th>
              <th class="col-size">Size</th>
              <th class="col-date">Modified</th>
            </tr>
          </thead>
          <tbody>
            {#each outputFiles as f (f.path)}
              <tr
                class="picker-row"
                class:picked={importSelected.has(f.path)}
                onclick={() => toggleImportFile(f.path)}
              >
                <td class="col-check">
                  <input
                    type="checkbox"
                    checked={importSelected.has(f.path)}
                    onclick={(e) => e.stopPropagation()}
                    onchange={() => toggleImportFile(f.path)}
                  />
                </td>
                <td class="col-name mono-cell">{f.name}</td>
                <td class="col-size dim-cell">{formatSize(f.size)}</td>
                <td class="col-date dim-cell">{f.modTime.slice(0, 10)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <div class="picker-footer">
        <div class="dest-row">
          <span class="dest-label">Destination</span>
          <span class="dest-root">{rootFolder}/</span>
          <input
            class="dest-input"
            type="text"
            placeholder="subfolder (optional)"
            bind:value={importSubfolder}
            spellcheck="false"
          />
        </div>
        {#if importError}<p class="form-error">{importError}</p>{/if}
        <div class="picker-actions">
          <button
            class="btn-primary"
            onclick={doImportOutputs}
            disabled={importing || importSelected.size === 0 || !rootFolder}
          >
            {importing ? "Copying…" : `Copy ${importSelected.size} file${importSelected.size !== 1 ? "s" : ""} to NAS`}
          </button>
          <button class="btn-ghost" onclick={() => (showImport = false)}>Cancel</button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  /* ── Layout ─────────────────────────────────────────────────────────────── */

  .projects-layout {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  /* ── Sidebar ─────────────────────────────────────────────────────────────── */

  .project-sidebar {
    width: 220px;
    flex-shrink: 0;
    background: var(--bg-panel);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 12px 8px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .sidebar-title {
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .sidebar-hint {
    font-size: 0.78rem;
    color: var(--text-secondary);
    padding: 16px 12px;
    line-height: 1.5;
    margin: 0;
  }

  .create-form {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex-shrink: 0;
  }

  .create-input {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    font-size: 0.82rem;
    padding: 4px 8px;
    outline: none;
    width: 100%;
  }

  .create-input:focus {
    border-color: var(--accent);
  }

  .create-actions {
    display: flex;
    gap: 6px;
  }

  .project-list {
    list-style: none;
    margin: 0;
    padding: 4px 0;
    overflow-y: auto;
    flex: 1;
  }

  .project-list::-webkit-scrollbar {
    width: 4px;
  }
  .project-list::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 2px;
  }

  .project-item {
    display: flex;
    flex-direction: column;
    padding: 7px 12px;
    cursor: pointer;
    border-left: 3px solid transparent;
    transition: background 0.12s;
  }

  .project-item:hover {
    background: var(--bg-row-hover);
  }

  .project-item.active {
    background: var(--bg-row-hover);
    border-left-color: var(--accent);
  }

  .project-name {
    font-size: 0.83rem;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .project-date {
    font-size: 0.7rem;
    color: var(--text-secondary);
    margin-top: 1px;
  }

  /* ── Detail panel ────────────────────────────────────────────────────────── */

  .project-detail {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-base);
  }

  .empty-detail {
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
    font-size: 1rem;
    font-weight: 500;
    color: var(--text-primary);
    margin: 0;
  }

  .empty-sub {
    font-size: 0.82rem;
    color: var(--text-secondary);
    margin: 0;
  }

  .detail-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: 18px 24px 14px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-panel);
    flex-shrink: 0;
    gap: 16px;
  }

  .detail-meta {
    flex: 1;
    min-width: 0;
  }

  .detail-name {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0 0 4px;
  }

  .detail-desc {
    font-size: 0.82rem;
    color: var(--text-secondary);
    margin: 0 0 4px;
  }

  .detail-folder {
    font-size: 0.75rem;
    color: var(--text-secondary);
    font-family: monospace;
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .detail-actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
    align-items: flex-start;
  }

  /* ── Detail body ─────────────────────────────────────────────────────────── */

  .detail-body {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0;
  }

  .detail-body::-webkit-scrollbar {
    width: 6px;
  }
  .detail-body::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

  /* ── Collapsible sections ────────────────────────────────────────────────── */

  .detail-section {
    border-bottom: 1px solid var(--border);
  }

  .section-hdr-row {
    display: flex;
    align-items: center;
    background: var(--bg-panel);
    padding-right: 12px;
  }

  .section-hdr-row:hover > .section-hdr {
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
  }

  .section-hdr {
    display: flex;
    align-items: center;
    gap: 7px;
    flex: 1;
    padding: 10px 24px;
    background: var(--bg-panel);
    border: none;
    cursor: pointer;
    user-select: none;
    text-align: left;
    transition: background 0.12s;
  }

  .section-hdr:hover {
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
  }


  .section-hdr.no-toggle {
    cursor: default;
  }

  .section-hdr.no-toggle:hover {
    background: var(--bg-panel);
  }

  .section-chevron {
    font-size: 0.6rem;
    color: var(--text-secondary);
    width: 10px;
    flex-shrink: 0;
  }

  .section-title {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .section-count {
    font-size: 0.75rem;
    color: var(--accent);
    font-weight: 400;
  }

  .section-spacer {
    flex: 1;
  }

  /* Live-watch indicator */
  .pulse-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent);
    opacity: 0.7;
    animation: pulse 2.5s ease-in-out infinite;
    flex-shrink: 0;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.3; }
    50% { opacity: 0.9; }
  }

  .section-body {
    padding: 10px 24px 14px;
  }

  /* ── File tables ─────────────────────────────────────────────────────────── */

  .hint {
    font-size: 0.82rem;
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.5;
  }

  .hint code {
    font-family: monospace;
    color: var(--accent);
    font-size: 0.78rem;
  }

  .file-scroll {
    border: 1px solid var(--border);
    border-radius: 5px;
    overflow-y: auto;
    max-height: 220px;
  }

  .file-scroll::-webkit-scrollbar {
    width: 6px;
  }
  .file-scroll::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

  .file-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }

  .file-table thead th {
    padding: 5px 10px;
    text-align: left;
    font-size: 0.7rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-secondary);
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
  }

  .file-table td {
    padding: 5px 10px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
  }

  .file-table tr:last-child td {
    border-bottom: none;
  }

  .mono-cell {
    font-family: monospace;
  }

  .dim-cell {
    color: var(--text-secondary) !important;
    font-size: 0.78rem;
  }

  .col-size {
    text-align: right;
    width: 72px;
  }

  .col-date {
    width: 90px;
    font-variant-numeric: tabular-nums;
  }

  /* ── Modals (shared) ─────────────────────────────────────────────────────── */

  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 500;
  }

  .modal {
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    padding: 24px;
    min-width: 320px;
    max-width: 440px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.6);
  }

  .modal-title {
    font-size: 0.95rem;
    color: var(--text-primary);
    margin: 0;
  }

  .modal-sub {
    font-size: 0.82rem;
    color: var(--text-secondary);
    margin: 0;
  }

  .modal-actions {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }

  /* ── Picker modal ────────────────────────────────────────────────────────── */

  .picker-modal {
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    width: 700px;
    max-width: 92vw;
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.6);
    overflow: hidden;
  }

  .picker-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 16px 10px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .picker-title {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--text-primary);
  }

  .picker-toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .picker-search {
    flex: 1;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    font-size: 0.82rem;
    padding: 4px 8px;
    outline: none;
  }

  .picker-search:focus {
    border-color: var(--accent);
  }

  .picker-count {
    font-size: 0.78rem;
    color: var(--text-secondary);
    white-space: nowrap;
  }

  .picker-list-scroll {
    flex: 1;
    overflow-y: auto;
  }

  .picker-list-scroll::-webkit-scrollbar {
    width: 6px;
  }
  .picker-list-scroll::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

  .picker-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }

  .picker-table thead th {
    padding: 5px 8px;
    text-align: left;
    font-size: 0.7rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    position: sticky;
    top: 0;
  }

  .picker-row {
    cursor: pointer;
    transition: background 0.1s;
  }

  .picker-row:hover {
    background: var(--bg-row-hover);
  }

  .picker-row.picked {
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
  }

  .picker-row td {
    padding: 5px 8px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .col-check {
    width: 32px;
    text-align: center;
  }

  .col-check input {
    accent-color: var(--accent);
    cursor: pointer;
  }

  .col-name {
    font-family: monospace;
  }

  .picker-footer {
    padding: 10px 16px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 8px;
    flex-shrink: 0;
  }

  .mode-toggle {
    display: flex;
    gap: 16px;
    font-size: 0.82rem;
    color: var(--text-secondary);
  }

  .mode-opt {
    display: flex;
    align-items: center;
    gap: 5px;
    cursor: pointer;
  }

  .mode-opt input {
    accent-color: var(--accent);
    cursor: pointer;
  }

  .picker-actions {
    display: flex;
    gap: 8px;
  }

  /* ── Destination row (import modal) ──────────────────────────────────────── */

  .dest-row {
    display: flex;
    align-items: center;
    gap: 4px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 4px 8px;
    font-size: 0.82rem;
  }

  .dest-label {
    font-size: 0.75rem;
    color: var(--text-secondary);
    flex-shrink: 0;
    margin-right: 4px;
  }

  .dest-root {
    color: var(--text-secondary);
    font-family: monospace;
    flex-shrink: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 200px;
  }

  .dest-input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    color: var(--text-primary);
    font-family: monospace;
    font-size: 0.82rem;
    min-width: 0;
  }

  /* ── Shared ──────────────────────────────────────────────────────────────── */

  .form-error {
    font-size: 0.75rem;
    color: var(--danger);
    margin: 0;
  }

  .small {
    font-size: 0.75rem;
    padding: 2px 8px;
  }

  .hint.padded {
    padding: 16px;
  }

  /* ── Object picker step ──────────────────────────────────────────────────── */

  .object-list {
    display: flex;
    flex-direction: column;
    padding: 6px 0;
  }

  .object-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 16px;
    cursor: pointer;
    font-size: 0.85rem;
    color: var(--text-secondary);
    transition: background 0.1s;
  }

  .object-item:hover {
    background: var(--bg-row-hover);
    color: var(--text-primary);
  }

  .object-item input[type="checkbox"] {
    accent-color: var(--accent);
    cursor: pointer;
    flex-shrink: 0;
  }

  .object-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* ── Load more row ───────────────────────────────────────────────────────── */

  .load-more-row {
    display: flex;
    justify-content: center;
    padding: 10px 0;
    border-top: 1px solid var(--border);
  }
</style>
