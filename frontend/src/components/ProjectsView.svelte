<script lang="ts">
  import { untrack } from "svelte";
  import {
    ListProjects,
    GetProjectLibraryFrames,
    GetProjectOutputFiles,
    RemoveFramesFromProject,
    OpenFolder,
  } from "$app";
  import type * as app from "$models/app";
  import { ui } from "../lib/uiState.svelte";
  import { toast } from "../lib/toast.svelte";
  import { describeAddResult, type ProjectFrameType } from "../lib/projects/frames";
  import { plural } from "../lib/utils";
  import ProjectSidebar from "./projects/ProjectSidebar.svelte";
  import ProjectHeader from "./projects/ProjectHeader.svelte";
  import ProjectFrames from "./projects/ProjectFrames.svelte";
  import ProjectOutputs from "./projects/ProjectOutputs.svelte";
  import CreateProjectModal from "./projects/CreateProjectModal.svelte";
  import RemoveProjectModal from "./projects/RemoveProjectModal.svelte";
  import AddFramesModal from "./projects/AddFramesModal.svelte";
  import SaveOutputsModal from "./projects/SaveOutputsModal.svelte";

  interface Props {
    rootFolder: string;
    projectsFolder: string;
    /** True while this view's tab is visible. */
    active?: boolean;
    /** Request a library (re)scan/index. */
    onscan?: () => void;
    /** Siril CLI is available. */
    sirilAvailable?: boolean;
  }

  let {
    rootFolder,
    projectsFolder,
    active = true,
    onscan,
    sirilAvailable = false,
  }: Props = $props();

  // ── Project list ──────────────────────────────────────────────────────────
  let projects = $state<app.Project[]>([]);
  let loadingProjects = $state(false);
  let projectsError = $state("");
  let selected = $state<app.Project | null>(null);

  /** Project to select once the list (re)loads — set by selectProjectById. */
  let pendingSelectId: number | null = null;
  let loadPromise: Promise<void> | null = null;

  function loadProjects(): Promise<void> {
    if (loadPromise) return loadPromise;
    loadPromise = (async () => {
      loadingProjects = true;
      projectsError = "";
      try {
        const list = (await ListProjects()) ?? [];
        projects = list;
        const wantId = pendingSelectId ?? selected?.id ?? ui.lastProjectId;
        const p = wantId != null ? list.find((x) => x.id === wantId) : undefined;
        if (p) {
          if (pendingSelectId === p.id) pendingSelectId = null;
          if (p.id !== selected?.id) void selectProject(p);
          else selected = p;
        } else if (selected) {
          clearSelection();
        }
      } catch (e) {
        projectsError = String(e);
      } finally {
        loadingProjects = false;
        loadPromise = null;
      }
    })();
    return loadPromise;
  }

  // Load on mount, and again if the projects folder setting changes.
  $effect(() => {
    void projectsFolder;
    untrack(() => void loadProjects());
  });

  /** Selects a project by id, (re)loading the list first if it isn't known yet. */
  export function selectProjectById(id: number): void {
    const p = projects.find((x) => x.id === id);
    if (p) {
      pendingSelectId = null;
      void selectProject(p);
      return;
    }
    pendingSelectId = id;
    void (async () => {
      if (loadPromise) await loadPromise;
      if (pendingSelectId !== id) return;
      await loadProjects();
      if (pendingSelectId === id) pendingSelectId = null; // unknown id: give up quietly
    })();
  }

  // ── Selected project's frames ─────────────────────────────────────────────
  let frames = $state<app.LibraryFrame[]>([]);
  let framesLoading = $state(false);
  let framesError = $state("");
  let framesReq = 0;

  function selectProject(p: app.Project): Promise<void> {
    if (selected?.id !== p.id) {
      frames = [];
      framesError = "";
    }
    selected = p;
    ui.lastProjectId = p.id;
    return reloadFrames();
  }

  function clearSelection() {
    framesReq++;
    selected = null;
    frames = [];
    framesError = "";
    framesLoading = false;
  }

  /** Reloads the selected project's frames; responses for a previous selection are dropped. */
  async function reloadFrames(): Promise<void> {
    const proj = selected;
    if (!proj) return;
    const req = ++framesReq;
    framesLoading = true;
    framesError = "";
    try {
      const list = await GetProjectLibraryFrames(proj.folder);
      if (req !== framesReq) return;
      frames = list ?? [];
    } catch (e) {
      if (req !== framesReq) return;
      framesError = String(e);
    } finally {
      if (req === framesReq) framesLoading = false;
    }
  }

  let projectFramePaths = $derived(new Set(frames.map((f) => f.nasPath)));

  async function removeFrames(paths: string[]) {
    const proj = selected;
    if (!proj) return;
    try {
      await RemoveFramesFromProject(proj.folder, paths);
      toast.success(`Removed ${plural(paths.length, "frame")} from the project`);
    } catch (e) {
      toast.error(`Couldn't remove frames: ${String(e)}`);
      throw e;
    } finally {
      if (selected?.id === proj.id) await reloadFrames();
    }
  }

  // ── Outputs (polled while visible) ────────────────────────────────────────
  let outputs = $state<app.ProjectOutputFile[]>([]);
  let outputsLoaded = $state(false);
  let outputsError = $state("");
  let outputsFolder = "";

  $effect(() => {
    const folder = selected?.folder ?? "";
    const visible = active !== false;
    if (folder !== outputsFolder) {
      outputsFolder = folder;
      outputs = [];
      outputsLoaded = false;
      outputsError = "";
    }
    if (!folder || !visible) return;
    let cancelled = false;
    const poll = async () => {
      try {
        const files = await GetProjectOutputFiles(folder);
        if (cancelled) return;
        outputs = files ?? [];
        outputsError = "";
      } catch (e) {
        if (cancelled) return;
        outputsError = String(e);
      } finally {
        if (!cancelled) outputsLoaded = true;
      }
    };
    void poll();
    const id = setInterval(poll, 3000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  });

  // ── Modals ────────────────────────────────────────────────────────────────
  let showCreate = $state(false);
  let removeTarget = $state<app.Project | null>(null);
  let pickerType = $state<ProjectFrameType | null>(null);
  let showSave = $state(false);

  function onCreated(p: app.Project) {
    showCreate = false;
    projects = [p, ...projects.filter((x) => x.id !== p.id)];
    void selectProject(p);
    toast.success(`Created project “${p.name}”`);
  }

  function onRemoved(p: app.Project) {
    removeTarget = null;
    projects = projects.filter((x) => x.id !== p.id);
    if (selected?.id === p.id) clearSelection();
    if (ui.lastProjectId === p.id) ui.lastProjectId = null;
    toast.success(`Removed “${p.name}” from the list. Its folder is still on disk.`, {
      action: { label: "Show folder", run: () => OpenFolder(p.folder) },
      timeout: 8000,
    });
  }

  function onAdded(result: app.AddFramesResult) {
    pickerType = null;
    const msg = describeAddResult(result);
    if (result.added > 0) toast.success(msg);
    else toast.info(msg);
    void reloadFrames();
  }
</script>

<div class="projects-layout">
  <ProjectSidebar
    {projects}
    selectedId={selected?.id ?? null}
    loading={loadingProjects}
    error={projectsError}
    {projectsFolder}
    onselect={(p) => void selectProject(p)}
    oncreate={() => (showCreate = true)}
    onretry={() => void loadProjects()}
  />

  <main class="project-detail">
    {#if !selected}
      <div class="empty-detail">
        <div class="empty-icon" aria-hidden="true">◫</div>
        {#if !projectsFolder}
          <p class="empty-title">No Projects folder</p>
          <p class="empty-sub">Choose where Siril projects are created in Settings.</p>
        {:else}
          <p class="empty-title">Select a project</p>
          <p class="empty-sub">Choose a project from the list, or create a new one.</p>
          <button class="btn-secondary" onclick={() => (showCreate = true)}>New project…</button>
        {/if}
      </div>
    {:else}
      <ProjectHeader
        project={selected}
        {sirilAvailable}
        onremove={() => (removeTarget = selected)}
      />

      <!-- The one scroll area for the project: frames, then outputs. -->
      <div class="detail-body">
        <ProjectFrames
          projectId={selected.id}
          {frames}
          loading={framesLoading}
          error={framesError}
          onretry={() => void reloadFrames()}
          onadd={(t) => (pickerType = t)}
          onremove={removeFrames}
        />
        <ProjectOutputs
          folder={selected.folder}
          files={outputs}
          loaded={outputsLoaded}
          error={outputsError}
          onsave={() => (showSave = true)}
        />
      </div>
    {/if}
  </main>
</div>

{#if showCreate}
  <CreateProjectModal {projectsFolder} onclose={() => (showCreate = false)} oncreated={onCreated} />
{/if}

{#if removeTarget}
  <RemoveProjectModal
    project={removeTarget}
    onclose={() => (removeTarget = null)}
    onremoved={onRemoved}
  />
{/if}

{#if pickerType && selected}
  <AddFramesModal
    {rootFolder}
    project={selected}
    {projectFramePaths}
    initialType={pickerType}
    onclose={() => (pickerType = null)}
    onadded={onAdded}
    {onscan}
  />
{/if}

{#if showSave && selected}
  <SaveOutputsModal
    {rootFolder}
    project={selected}
    files={outputs}
    onclose={() => (showSave = false)}
  />
{/if}

<style>
  .projects-layout {
    flex: 1;
    display: flex;
    overflow: hidden;
    min-height: 0;
  }

  .project-detail {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-base);
  }

  .detail-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .detail-body::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }
  .detail-body::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 4px;
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
    color: var(--accent);
    margin-bottom: 8px;
  }
  .empty-title {
    font-size: var(--fs-lg);
    font-weight: 500;
    color: var(--text-primary);
    margin: 0;
  }
  .empty-sub {
    font-size: var(--fs-md);
    color: var(--text-secondary);
    margin: 0 0 8px;
  }
</style>
