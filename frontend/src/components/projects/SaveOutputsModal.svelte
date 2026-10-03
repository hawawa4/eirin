<script lang="ts">
  import { SvelteSet } from "svelte/reactivity";
  import type * as app from "$models/app";
  import { ImportOutputFiles, OpenFolder } from "$app";
  import Modal from "../Modal.svelte";
  import { toast } from "../../lib/toast.svelte";
  import { joinPath, safeSubfolderName } from "../../lib/projects/frames";
  import { formatBytes, plural } from "../../lib/utils";

  interface Props {
    rootFolder: string;
    project: app.Project;
    files: app.ProjectOutputFile[];
    onclose: () => void;
  }
  let { rootFolder, project, files, onclose }: Props = $props();

  // Snapshot: the outputs list keeps polling while the dialog is open.
  // svelte-ignore state_referenced_locally
  const initialFiles = files.slice();
  const selected = new SvelteSet<string>(initialFiles.map((f) => f.path));
  // svelte-ignore state_referenced_locally
  let subfolder = $state(safeSubfolderName(project.name));
  let saving = $state(false);
  let error = $state("");
  /** Names that already exist at the destination (reported by the backend; nothing was copied). */
  let conflicts = $state<string[]>([]);

  let subfolderInvalid = $derived(subfolder.split(/[\\/]/).some((seg) => seg.trim() === ".."));
  let dest = $derived(joinPath(rootFolder, subfolder));
  let allSelected = $derived(selected.size === initialFiles.length && initialFiles.length > 0);

  function toggle(path: string) {
    if (selected.has(path)) selected.delete(path);
    else selected.add(path);
  }
  function toggleAll() {
    if (allSelected) selected.clear();
    else for (const f of initialFiles) selected.add(f.path);
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (saving || selected.size === 0 || !rootFolder || subfolderInvalid) return;
    saving = true;
    error = "";
    conflicts = [];
    const target = dest;
    const n = selected.size;
    try {
      const res = await ImportOutputFiles([...selected], target);
      if (res.conflicts?.length) {
        conflicts = res.conflicts;
        return;
      }
      toast.success(`Saved ${plural(res.copied?.length ?? n, "file")} to the library`, {
        action: { label: "Show folder", run: () => OpenFolder(target) },
      });
      onclose();
    } catch (err) {
      error = String(err);
    } finally {
      saving = false;
    }
  }
</script>

<Modal
  title="Save outputs to library"
  {onclose}
  busy={saving}
  dismissOnBackdrop={false}
  width="min(680px, 94vw)"
>
  <p class="intro">
    Copies the selected files from the project folder into your library and indexes them. Existing
    library files are never overwritten.
  </p>

  <form id="save-outputs-form" class="form" onsubmit={submit}>
    <div class="list-toolbar">
      <label class="check-all">
        <input type="checkbox" checked={allSelected} onchange={toggleAll} />
        Select all
      </label>
      <span class="count">{selected.size} of {initialFiles.length} selected</span>
    </div>
    <div class="file-list">
      {#each initialFiles as f (f.path)}
        <label class="file-row" class:picked={selected.has(f.path)}>
          <input type="checkbox" checked={selected.has(f.path)} onchange={() => toggle(f.path)} />
          <span class="name" class:conflict={conflicts.includes(f.name)}>{f.name}</span>
          <span class="dim">{formatBytes(f.size)}</span>
          <span class="dim">{f.modTime.slice(0, 10)}</span>
        </label>
      {/each}
    </div>

    <label class="dest-row">
      <span class="dest-label">Save to</span>
      <span class="dest-root" title={rootFolder}>{"‎" + rootFolder + "/‎"}</span>
      <input
        type="text"
        placeholder="subfolder (optional)"
        bind:value={subfolder}
        spellcheck="false"
        data-autofocus
        oninput={() => {
          conflicts = [];
          error = "";
        }}
      />
    </label>
    {#if subfolderInvalid}
      <p class="error">The subfolder can't contain “..”.</p>
    {/if}

    {#if conflicts.length > 0}
      <div class="error-box" role="alert">
        <strong>Nothing was saved.</strong> These files already exist in
        <code>{dest}</code>:
        <ul>
          {#each conflicts as c (c)}<li>{c}</li>{/each}
        </ul>
        Choose another subfolder, or deselect these files.
      </div>
    {:else if error}
      <p class="error" role="alert">Couldn't save: {error}</p>
    {/if}
  </form>

  {#snippet actions()}
    <button type="button" class="btn-ghost" disabled={saving} onclick={onclose}>Cancel</button>
    <button
      type="submit"
      form="save-outputs-form"
      class="btn-primary"
      disabled={saving || selected.size === 0 || !rootFolder || subfolderInvalid}
    >
      {saving ? "Saving…" : `Save ${plural(selected.size, "file")}`}
    </button>
  {/snippet}
</Modal>

<style>
  .intro {
    margin: 0;
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .list-toolbar {
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
  }
  .count {
    color: var(--text-secondary);
  }
  .file-list {
    border: 1px solid var(--border);
    border-radius: 5px;
    max-height: min(320px, 40vh);
    overflow-y: auto;
  }
  .file-row {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    gap: 10px;
    align-items: center;
    padding: 5px 10px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    color: var(--text-primary);
  }
  .file-row:last-child {
    border-bottom: none;
  }
  .file-row:hover {
    background: var(--bg-row-hover);
  }
  .file-row.picked {
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
  }
  input[type="checkbox"] {
    accent-color: var(--accent);
  }
  .name {
    font-family: monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .name.conflict {
    color: var(--danger);
  }
  .dim {
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
  .dest-row {
    display: flex;
    align-items: center;
    gap: 4px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 5px 8px;
  }
  .dest-row:focus-within {
    border-color: var(--accent);
  }
  .dest-label {
    color: var(--text-secondary);
    flex-shrink: 0;
    margin-right: 6px;
  }
  .dest-root {
    color: var(--text-secondary);
    font-family: monospace;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    direction: rtl;
    text-align: left;
    max-width: 45%;
  }
  .dest-row input {
    flex: 1;
    min-width: 0;
    background: transparent;
    border: none;
    outline: none;
    color: var(--text-primary);
    font-family: monospace;
    font-size: var(--fs-sm);
  }
  .error {
    margin: 0;
    color: var(--danger);
    word-break: break-word;
  }
  .error-box {
    border: 1px solid var(--danger);
    border-radius: 5px;
    padding: 8px 12px;
    color: var(--text-primary);
    background: color-mix(in srgb, var(--danger) 10%, var(--bg-panel));
  }
  .error-box strong {
    color: var(--danger);
  }
  .error-box code {
    font-family: monospace;
    word-break: break-all;
  }
  .error-box ul {
    margin: 6px 0;
    padding-left: 18px;
    font-family: monospace;
  }
</style>
