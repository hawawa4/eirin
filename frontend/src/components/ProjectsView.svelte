<script lang="ts">
  import { onMount } from "svelte";
  import {
    ListProjects,
    CreateProject,
    DeleteProject,
    GetProjectFrames,
    AddFramesToProject,
    OpenProjectInSiril,
    GetLibraryFrames,
  } from "../../wailsjs/go/app/App.js";
  import type { app } from "../../wailsjs/go/models";
  import type { Project } from "../lib/types";

  interface Props {
    rootFolder: string;
    projectsFolder: string;
  }

  let { rootFolder, projectsFolder }: Props = $props();

  // ── Project list ──────────────────────────────────────────────────────────
  let projects = $state<Project[]>([]);
  let selected = $state<Project | null>(null);
  let loadingProjects = $state(false);

  // ── Project frames (lights/ folder scan) ──────────────────────────────────
  let projectFrames = $state<string[]>([]);
  let loadingFrames = $state(false);

  // ── Create form ───────────────────────────────────────────────────────────
  let showCreate = $state(false);
  let createName = $state("");
  let createDesc = $state("");
  let creating = $state(false);
  let createError = $state("");

  // ── Frame picker ──────────────────────────────────────────────────────────
  let showPicker = $state(false);
  let allLights = $state<app.LibraryFrame[]>([]);
  let pickerFilter = $state("");
  let pickerSelected = $state<Set<string>>(new Set());
  let addMode = $state<"symlink" | "copy">("symlink");
  let adding = $state(false);
  let addError = $state("");

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

  async function openPicker() {
    pickerFilter = "";
    pickerSelected = new Set();
    addError = "";
    allLights = rootFolder
      ? (await GetLibraryFrames(rootFolder)).filter((f) => f.frameType === "light")
      : [];
    showPicker = true;
  }

  let filteredLights = $derived(
    allLights.filter(
      (f) =>
        !pickerFilter ||
        f.fileName.toLowerCase().includes(pickerFilter.toLowerCase()) ||
        f.object.toLowerCase().includes(pickerFilter.toLowerCase()),
    ),
  );

  function togglePick(nasPath: string) {
    const next = new Set(pickerSelected);
    if (next.has(nasPath)) next.delete(nasPath);
    else next.add(nasPath);
    pickerSelected = next;
  }

  function toggleAll() {
    if (pickerSelected.size === filteredLights.length) {
      pickerSelected = new Set();
    } else {
      pickerSelected = new Set(filteredLights.map((f) => f.nasPath));
    }
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
        {#if createError}
          <p class="form-error">{createError}</p>
        {/if}
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
      <div class="detail-header">
        <div class="detail-meta">
          <h2 class="detail-name">{selected.name}</h2>
          {#if selected.description}
            <p class="detail-desc">{selected.description}</p>
          {/if}
          <p class="detail-folder" title={selected.folder}>{selected.folder}</p>
        </div>
        <div class="detail-actions">
          <button class="btn-primary siril-btn" onclick={doOpenInSiril} title="Open project folder in Siril">
            Open in Siril
          </button>
          <button
            class="btn-ghost danger-btn"
            onclick={() => (confirmDelete = selected)}
            title="Remove project from list (does not delete files)"
          >
            Remove
          </button>
        </div>
      </div>

      <!-- Lights section -->
      <div class="lights-section">
        <div class="lights-header">
          <span class="lights-title">
            Lights
            {#if !loadingFrames}
              <span class="lights-count">{projectFrames.length}</span>
            {/if}
          </span>
          <button class="btn-secondary" onclick={openPicker}>Add frames…</button>
        </div>

        {#if loadingFrames}
          <p class="hint">Loading…</p>
        {:else if projectFrames.length === 0}
          <p class="hint">No frames yet. Click "Add frames" to link light frames from your library.</p>
        {:else}
          <div class="frame-list-scroll">
            <table class="frame-table">
              <thead>
                <tr><th>Filename</th></tr>
              </thead>
              <tbody>
                {#each projectFrames as name (name)}
                  <tr><td class="frame-name">{name}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    {/if}
  </main>
</div>

<!-- ── Delete confirm modal ──────────────────────────────────────────────────── -->
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

<!-- ── Frame picker modal ─────────────────────────────────────────────────────── -->
{#if showPicker}
  <div class="modal-backdrop" onclick={() => (showPicker = false)}>
    <div class="picker-modal" onclick={(e) => e.stopPropagation()}>
      <div class="picker-header">
        <span class="picker-title">Add light frames to "{selected?.name}"</span>
        <button class="btn-ghost" onclick={() => (showPicker = false)}>✕</button>
      </div>

      <div class="picker-toolbar">
        <input
          class="picker-search"
          type="search"
          placeholder="Filter by name or object…"
          bind:value={pickerFilter}
        />
        <button class="btn-ghost small" onclick={toggleAll}>
          {pickerSelected.size === filteredLights.length && filteredLights.length > 0
            ? "Deselect all"
            : "Select all"}
        </button>
        <span class="picker-count">{pickerSelected.size} selected</span>
      </div>

      <div class="picker-list-scroll">
        {#if allLights.length === 0}
          <p class="hint">No indexed light frames found. Run Build Index first.</p>
        {:else if filteredLights.length === 0}
          <p class="hint">No frames match the filter.</p>
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
                  <td class="col-obj">{f.object || "—"}</td>
                  <td class="col-filt">{f.filter || "—"}</td>
                  <td class="col-date">{f.dateObs ? f.dateObs.slice(0, 10) : "—"}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      <div class="picker-footer">
        <div class="mode-toggle">
          <label class="mode-opt">
            <input type="radio" name="addMode" value="symlink" bind:group={addMode} />
            Symlink
          </label>
          <label class="mode-opt">
            <input type="radio" name="addMode" value="copy" bind:group={addMode} />
            Copy
          </label>
        </div>
        {#if addError}
          <p class="form-error">{addError}</p>
        {/if}
        <div class="picker-actions">
          <button
            class="btn-primary"
            onclick={doAddFrames}
            disabled={adding || pickerSelected.size === 0}
          >
            {adding ? "Adding…" : `Add ${pickerSelected.size} frame${pickerSelected.size !== 1 ? "s" : ""}`}
          </button>
          <button class="btn-ghost" onclick={() => (showPicker = false)}>Cancel</button>
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
  }

  /* ── Create form ─────────────────────────────────────────────────────────── */

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

  .form-error {
    font-size: 0.75rem;
    color: var(--danger);
    margin: 0;
  }

  /* ── Project list ────────────────────────────────────────────────────────── */

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

  .danger-btn {
    color: var(--danger) !important;
    border-color: var(--danger) !important;
  }

  /* ── Lights section ──────────────────────────────────────────────────────── */

  .lights-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 16px 24px;
    gap: 10px;
  }

  .lights-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-shrink: 0;
  }

  .lights-title {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .lights-count {
    font-weight: 400;
    color: var(--accent);
    font-size: 0.78rem;
  }

  .hint {
    font-size: 0.82rem;
    color: var(--text-secondary);
    margin: 0;
  }

  .frame-list-scroll {
    flex: 1;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: 5px;
  }

  .frame-list-scroll::-webkit-scrollbar {
    width: 6px;
  }
  .frame-list-scroll::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 3px;
  }

  .frame-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.82rem;
  }

  .frame-table thead th {
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

  .frame-name {
    padding: 5px 10px;
    color: var(--text-primary);
    font-family: monospace;
    border-bottom: 1px solid var(--border);
  }

  /* ── Modals ──────────────────────────────────────────────────────────────── */

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

  /* ── Frame picker modal ──────────────────────────────────────────────────── */

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

  .col-obj,
  .col-filt,
  .col-date {
    color: var(--text-secondary) !important;
    font-size: 0.78rem;
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

  .small {
    font-size: 0.75rem;
    padding: 2px 8px;
  }
</style>
