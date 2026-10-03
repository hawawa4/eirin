<script lang="ts">
  import type * as app from "$models/app";
  import { DeleteProject } from "$app";
  import Modal from "../Modal.svelte";

  interface Props {
    project: app.Project;
    onclose: () => void;
    onremoved: (p: app.Project) => void;
  }
  let { project, onclose, onremoved }: Props = $props();

  let removing = $state(false);
  let error = $state("");

  async function remove() {
    removing = true;
    error = "";
    try {
      await DeleteProject(project.id);
      onremoved(project);
    } catch (e) {
      error = String(e);
    } finally {
      removing = false;
    }
  }
</script>

<Modal title="Remove “{project.name}”?" {onclose} busy={removing} danger width="min(480px, 92vw)">
  <p>
    The project is removed from Eirin's list. <strong>Nothing is deleted from disk</strong> — the project
    folder, its frame links and any outputs stay where they are:
  </p>
  <p class="path">{project.folder}</p>
  <p>You can delete the folder yourself from your file manager if you no longer need it.</p>
  {#if error}<p class="error" role="alert">Couldn't remove the project: {error}</p>{/if}

  {#snippet actions()}
    <button class="btn-ghost" data-autofocus disabled={removing} onclick={onclose}>Cancel</button>
    <button class="btn-danger" disabled={removing} onclick={remove}>
      {removing ? "Removing…" : "Remove from list"}
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
</style>
