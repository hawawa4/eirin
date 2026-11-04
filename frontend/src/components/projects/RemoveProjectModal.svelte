<script lang="ts">
  import type * as app from "$models/app";
  import { DeleteProject } from "$app";
  import Modal from "../Modal.svelte";

  interface Props {
    project: app.Project;
    onclose: () => void;
    /** `folderDeleted`: the folder went too, not just the list entry. */
    onremoved: (p: app.Project, folderDeleted: boolean) => void;
  }
  let { project, onclose, onremoved }: Props = $props();

  let removing = $state(false);
  let error = $state("");
  let deleteFolder = $state(true);

  async function remove() {
    removing = true;
    error = "";
    try {
      await DeleteProject(project.id, deleteFolder);
      onremoved(project, deleteFolder);
    } catch (e) {
      error = String(e);
    } finally {
      removing = false;
    }
  }
</script>

<Modal
  title={deleteFolder ? `Delete “${project.name}”?` : `Remove “${project.name}” from the list?`}
  {onclose}
  busy={removing}
  danger
  width="min(500px, 92vw)"
>
  <p class="path">{project.folder}</p>
  <label class="opt">
    <input type="checkbox" bind:checked={deleteFolder} disabled={removing} />
    Also delete the project folder
  </label>
  {#if deleteFolder}
    <p>
      The folder and <strong>everything in it</strong> are deleted: frame links, copied frames, and any
      Siril outputs (stacks, processed images) you haven't saved to the library. This can't be undone.
    </p>
    <p class="safe">Your frames in the library aren't affected — the links only point at them.</p>
  {:else}
    <p>The project leaves Eirin's list; the folder stays on disk as it is.</p>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}

  {#snippet actions()}
    <button class="btn-ghost" data-autofocus disabled={removing} onclick={onclose}>Cancel</button>
    <button class="btn-danger" disabled={removing} onclick={remove}>
      {removing ? "Removing…" : deleteFolder ? "Delete project and folder" : "Remove from list"}
    </button>
  {/snippet}
</Modal>

<style>
  p {
    margin: 0;
  }
  strong {
    color: var(--text-primary);
  }
  .path {
    font-family: monospace;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    word-break: break-all;
  }
  .error {
    color: var(--danger);
  }
  .opt {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--text-primary);
    cursor: pointer;
  }
  .opt input {
    accent-color: var(--danger);
  }
  .safe {
    color: var(--text-secondary);
    font-size: var(--fs-sm);
  }
</style>
