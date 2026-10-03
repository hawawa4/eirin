<script lang="ts">
  import { untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import type * as app from "$models/app";
  import {
    AddFramesToProjectDetailed,
    GetLibraryFrames,
    GetLightFramesPaged,
    GetLightObjects,
  } from "$app";
  import Modal from "../Modal.svelte";
  import FrameTable from "../FrameTable.svelte";
  import {
    PROJECT_FRAME_TYPES,
    PROJECT_TYPE_LABEL,
    calibrationGroupKey,
    groupCalibration,
    plural,
    type PickerGroup,
    type ProjectFrameType,
  } from "../../lib/projects/frames";

  interface Props {
    rootFolder: string;
    project: app.Project;
    /** NAS paths already in the project: shown greyed out and not selectable. */
    projectFramePaths: ReadonlySet<string>;
    initialType?: ProjectFrameType;
    onclose: () => void;
    onadded: (result: app.AddFramesResult) => void;
    onscan?: () => void;
  }
  let {
    rootFolder,
    project,
    projectFramePaths,
    initialType = "light",
    onclose,
    onadded,
    onscan,
  }: Props = $props();

  // ── Step 1: frame type + groups ───────────────────────────────────────────
  let step = $state<"groups" | "frames">("groups");
  let type = $state<ProjectFrameType>(untrack(() => initialType));
  let groups = $state<PickerGroup[]>([]);
  let groupsLoading = $state(false);
  let groupsError = $state("");
  let groupSearch = $state("");
  const selectedGroups = new SvelteSet<string>();

  /** Whole-library frame list, fetched once and only for calibration types. */
  let library: app.LibraryFrame[] | null = null;
  async function libraryFrames(): Promise<app.LibraryFrame[]> {
    if (!library) library = (await GetLibraryFrames(rootFolder)) ?? [];
    return library;
  }

  let groupsReq = 0;
  async function loadGroups() {
    const t = type;
    const req = ++groupsReq;
    groups = [];
    groupsError = "";
    selectedGroups.clear();
    if (!rootFolder) return;
    groupsLoading = true;
    try {
      let result: PickerGroup[];
      if (t === "light") {
        const objects = (await GetLightObjects(rootFolder)) ?? [];
        result = objects.map((o) => ({ key: o, label: o || "(no object)" }));
      } else {
        result = groupCalibration((await libraryFrames()).filter((f) => f.frameType === t));
      }
      if (req === groupsReq) groups = result;
    } catch (e) {
      if (req === groupsReq) groupsError = String(e);
    } finally {
      if (req === groupsReq) groupsLoading = false;
    }
  }

  $effect(() => {
    void type;
    untrack(loadGroups);
  });

  let visibleGroups = $derived(
    groupSearch.trim()
      ? groups.filter((g) => g.label.toLowerCase().includes(groupSearch.trim().toLowerCase()))
      : groups,
  );
  let allVisibleSelected = $derived(
    visibleGroups.length > 0 && visibleGroups.every((g) => selectedGroups.has(g.key)),
  );

  function toggleGroup(key: string) {
    if (selectedGroups.has(key)) selectedGroups.delete(key);
    else selectedGroups.add(key);
  }
  function toggleAllGroups() {
    if (allVisibleSelected) for (const g of visibleGroups) selectedGroups.delete(g.key);
    else for (const g of visibleGroups) selectedGroups.add(g.key);
  }

  // ── Step 2: frames ────────────────────────────────────────────────────────
  let frames = $state<app.LibraryFrame[]>([]);
  let hasMore = $state(false);
  let framesLoading = $state(false);
  let framesError = $state("");
  let framesResetKey = $state(0);
  let selected = $state<Set<string>>(new Set());
  let framesReq = 0;

  async function loadFrames(reset: boolean): Promise<boolean> {
    const req = reset ? ++framesReq : framesReq;
    if (reset) {
      frames = [];
      hasMore = false;
      framesResetKey++;
    }
    framesLoading = true;
    framesError = "";
    try {
      if (type === "light") {
        const page = await GetLightFramesPaged(rootFolder, [...selectedGroups], frames.length);
        if (req !== framesReq) return false;
        frames = [...frames, ...(page.frames ?? [])];
        hasMore = page.hasMore;
      } else {
        const t = type;
        const all = await libraryFrames();
        if (req !== framesReq) return false;
        frames = all.filter((f) => f.frameType === t && selectedGroups.has(calibrationGroupKey(f)));
        hasMore = false;
      }
      return true;
    } catch (e) {
      if (req === framesReq) framesError = String(e);
      return false;
    } finally {
      if (req === framesReq) framesLoading = false;
    }
  }

  async function loadAll() {
    const req = framesReq;
    for (let i = 0; i < 1000 && hasMore && req === framesReq; i++) {
      if (!(await loadFrames(false))) break;
    }
  }

  function goToFrames(e?: SubmitEvent) {
    e?.preventDefault();
    if (selectedGroups.size === 0) return;
    step = "frames";
    selected = new Set();
    void loadFrames(true);
  }

  // ── Add ───────────────────────────────────────────────────────────────────
  let mode = $state<"symlink" | "copy">("symlink");
  let adding = $state(false);
  let addError = $state("");

  async function add() {
    if (selected.size === 0 || adding) return;
    adding = true;
    addError = "";
    try {
      const result = await AddFramesToProjectDetailed(project.folder, [...selected], mode);
      onadded(result);
    } catch (e) {
      addError = String(e);
    } finally {
      adding = false;
    }
  }

  let alreadyIn = $derived(frames.filter((f) => projectFramePaths.has(f.nasPath)).length);
  let dirty = $derived(selectedGroups.size > 0 || selected.size > 0 || adding);
  let typeLabel = $derived(PROJECT_TYPE_LABEL[type]);
</script>

<Modal
  title="Add frames to “{project.name}”"
  {onclose}
  busy={adding}
  dismissOnBackdrop={!dirty}
  width="min(880px, 94vw)"
>
  {#if step === "groups"}
    <form id="picker-groups-form" class="step" onsubmit={goToFrames}>
      <div class="type-row" role="group" aria-label="Frame type">
        {#each PROJECT_FRAME_TYPES as t (t)}
          <button
            type="button"
            class="tool-btn"
            class:active={type === t}
            aria-pressed={type === t}
            onclick={() => (type = t)}>{PROJECT_TYPE_LABEL[t].many}</button
          >
        {/each}
      </div>
      <p class="step-hint">
        {type === "light"
          ? "Choose the objects to pick light frames from."
          : type === "dark"
            ? "Darks are grouped by exposure and gain — pick the ones matching your lights."
            : type === "flat"
              ? "Flats are grouped by filter."
              : "Biases are grouped by gain."}
      </p>

      <div class="group-toolbar">
        <label class="check-all">
          <input
            type="checkbox"
            checked={allVisibleSelected}
            disabled={visibleGroups.length === 0}
            onchange={toggleAllGroups}
          />
          Select all
        </label>
        <input
          class="search"
          type="search"
          placeholder="Search…"
          aria-label="Search groups"
          bind:value={groupSearch}
          data-autofocus
        />
        <span class="count">{selectedGroups.size} selected</span>
      </div>

      <div class="group-list">
        {#if !rootFolder}
          <p class="state">No library folder is set. Choose one in Settings first.</p>
        {:else if groupsLoading}
          <p class="state">Loading…</p>
        {:else if groupsError}
          <div class="state error" role="alert">
            Couldn't load {typeLabel.one} frames: {groupsError}
            <button type="button" class="btn-secondary" onclick={loadGroups}>Retry</button>
          </div>
        {:else if groups.length === 0}
          <div class="state">
            <p>No indexed {typeLabel.one} frames found in your library.</p>
            {#if onscan}
              <button
                type="button"
                class="btn-secondary"
                onclick={() => {
                  onscan?.();
                  onclose();
                }}>Scan library</button
              >
            {/if}
          </div>
        {:else if visibleGroups.length === 0}
          <p class="state">Nothing matches “{groupSearch}”.</p>
        {:else}
          {#each visibleGroups as g (g.key)}
            <label class="group-item">
              <input
                type="checkbox"
                checked={selectedGroups.has(g.key)}
                onchange={() => toggleGroup(g.key)}
              />
              <span class="group-name">{g.label}</span>
              {#if g.count !== undefined}<span class="group-count">{plural(g.count, "frame")}</span
                >{/if}
            </label>
          {/each}
        {/if}
      </div>
    </form>
  {:else}
    <div class="step">
      <div class="frames-toolbar">
        <button type="button" class="btn-ghost small" onclick={() => (step = "groups")}
          >← Back</button
        >
        <span class="count">
          {typeLabel.many} · {selected.size} selected{alreadyIn > 0
            ? ` · ${alreadyIn} already in the project`
            : ""}
        </span>
      </div>
      <div class="frames-box">
        {#if framesError}
          <div class="state error" role="alert">
            Couldn't load frames: {framesError}
            <button type="button" class="btn-secondary" onclick={() => loadFrames(true)}
              >Retry</button
            >
          </div>
        {:else if framesLoading && frames.length === 0}
          <p class="state">Loading frames…</p>
        {:else}
          <FrameTable
            {frames}
            {hasMore}
            loadingMore={framesLoading}
            onloadmore={() => loadFrames(false)}
            onloadall={loadAll}
            onselectionchange={(s) => (selected = s)}
            hiddenColumns={["frameType"]}
            resetKey={framesResetKey}
            disabledPaths={projectFramePaths}
            disabledTitle="Already in this project"
          />
        {/if}
      </div>
      <div class="mode-row" role="radiogroup" aria-label="How to add frames">
        <label
          title="Link to the library file — no extra disk space (recommended for NAS libraries)"
        >
          <input type="radio" name="addMode" value="symlink" bind:group={mode} /> Symlink
        </label>
        <label title="Copy the file into the project folder">
          <input type="radio" name="addMode" value="copy" bind:group={mode} /> Copy
        </label>
      </div>
      {#if addError}<p class="error" role="alert">{addError}</p>{/if}
    </div>
  {/if}

  {#snippet actions()}
    <button type="button" class="btn-ghost" disabled={adding} onclick={onclose}>Cancel</button>
    {#if step === "groups"}
      <button
        type="submit"
        form="picker-groups-form"
        class="btn-primary"
        disabled={selectedGroups.size === 0 || groupsLoading}
      >
        Continue ({plural(selectedGroups.size, "group")})
      </button>
    {:else}
      <button
        type="button"
        class="btn-primary"
        disabled={adding || selected.size === 0}
        onclick={add}
      >
        {adding ? "Adding…" : `Add ${plural(selected.size, "frame")}`}
      </button>
    {/if}
  {/snippet}
</Modal>

<style>
  .step {
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-height: 0;
  }
  .type-row {
    display: flex;
    gap: 6px;
  }
  .type-row .tool-btn {
    font-size: var(--fs-sm);
    padding: 4px 12px;
  }
  .step-hint {
    margin: 0;
  }
  .group-toolbar,
  .frames-toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .check-all {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    color: var(--text-primary);
    white-space: nowrap;
  }
  .search {
    flex: 1;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    font-size: var(--fs-sm);
    padding: 4px 8px;
    outline: none;
  }
  .search:focus {
    border-color: var(--accent);
  }
  .count {
    color: var(--text-secondary);
    white-space: nowrap;
  }
  input[type="checkbox"],
  input[type="radio"] {
    accent-color: var(--accent);
    cursor: pointer;
  }

  .group-list {
    border: 1px solid var(--border);
    border-radius: 5px;
    height: min(46vh, 420px);
    overflow-y: auto;
  }
  .group-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 12px;
    cursor: pointer;
    font-size: var(--fs-md);
    color: var(--text-primary);
    border-bottom: 1px solid var(--border);
  }
  .group-item:hover {
    background: var(--bg-row-hover);
  }
  .group-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .group-count {
    color: var(--text-secondary);
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }

  .frames-box {
    display: flex;
    flex-direction: column;
    height: min(52vh, 520px);
    border: 1px solid var(--border);
    border-radius: 5px;
    overflow: hidden;
  }

  .state {
    margin: 0;
    padding: 16px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  .state p {
    margin: 0;
  }
  .state.error {
    color: var(--danger);
  }
  .mode-row {
    display: flex;
    gap: 16px;
    color: var(--text-primary);
  }
  .mode-row label {
    display: flex;
    align-items: center;
    gap: 5px;
    cursor: pointer;
  }
  .error {
    margin: 0;
    color: var(--danger);
    word-break: break-word;
  }
  .small {
    padding: 3px 10px;
    font-size: var(--fs-xs);
  }
</style>
