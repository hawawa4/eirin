<script lang="ts">
  import type * as app from "$models/app";
  import { CreateProject } from "$app";
  import Modal from "../Modal.svelte";
  import { joinPath, sanitizeFolderName } from "../../lib/projects/frames";

  interface Props {
    projectsFolder: string;
    onclose: () => void;
    oncreated: (p: app.Project) => void;
  }
  let { projectsFolder, onclose, oncreated }: Props = $props();

  let name = $state("");
  let description = $state("");
  let creating = $state(false);
  let error = $state("");

  let folderPreview = $derived(
    name.trim() ? joinPath(projectsFolder, sanitizeFolderName(name)) : "",
  );

  async function submit(e: SubmitEvent) {
    e.preventDefault();
    if (!name.trim() || creating) return;
    creating = true;
    error = "";
    try {
      const p = await CreateProject(name.trim(), description.trim());
      oncreated(p);
    } catch (err) {
      error = String(err);
    } finally {
      creating = false;
    }
  }
</script>

<Modal title="New project" {onclose} busy={creating} width="min(480px, 92vw)">
  <form id="create-project-form" class="form" onsubmit={submit}>
    <label class="field">
      <span>Name</span>
      <input
        type="text"
        bind:value={name}
        placeholder="e.g. M31 Andromeda"
        spellcheck="false"
        data-autofocus
        oninput={() => (error = "")}
      />
    </label>
    <label class="field">
      <span>Description <em>(optional)</em></span>
      <input type="text" bind:value={description} spellcheck="false" />
    </label>
    {#if folderPreview}
      <p class="preview">
        Folder: <code title={folderPreview}>{folderPreview}</code>
      </p>
    {/if}
    {#if error}
      <p class="error" role="alert">{error}</p>
    {/if}
  </form>

  {#snippet actions()}
    <button type="button" class="btn-ghost" disabled={creating} onclick={onclose}>Cancel</button>
    <button
      type="submit"
      form="create-project-form"
      class="btn-primary"
      disabled={creating || !name.trim()}>{creating ? "Creating…" : "Create project"}</button
    >
  {/snippet}
</Modal>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .field em {
    font-style: normal;
    color: var(--text-secondary);
  }
  .field input {
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    font-size: var(--fs-md);
    padding: 6px 8px;
    outline: none;
  }
  .field input:focus {
    border-color: var(--accent);
  }
  .preview {
    margin: 0;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    word-break: break-all;
  }
  .preview code {
    font-family: monospace;
    color: var(--text-primary);
  }
  .error {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--danger);
    word-break: break-word;
  }
</style>
