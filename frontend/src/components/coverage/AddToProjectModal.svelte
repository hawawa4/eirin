<script lang="ts">
  import { onMount } from "svelte";
  import { AddFramesToProjectDetailed, ListProjects } from "$app";
  import type { Project } from "../../lib/types";
  import { toast } from "../../lib/toast.svelte";
  import { describeAddResult } from "../../lib/projects/frames";
  import { plural } from "../../lib/utils";
  import Modal from "../Modal.svelte";

  interface Props {
    /** NAS paths of the lights to add. */
    paths: string[];
    onclose: () => void;
    /** Opens a project in the Projects view (offered once the frames are added). */
    onopenproject: (project: Project) => void;
  }

  let { paths, onclose, onopenproject }: Props = $props();

  let projects = $state<Project[]>([]);
  let loading = $state(true);
  let projectId = $state<number | null>(null);
  let mode = $state<"symlink" | "copy">("symlink");
  let error = $state("");
  let adding = $state(false);

  onMount(async () => {
    try {
      projects = (await ListProjects()) ?? [];
      projectId = projects[0]?.id ?? null;
    } catch (err) {
      error = `Couldn't list projects: ${String(err)}`;
    } finally {
      loading = false;
    }
  });

  async function submit(e?: Event) {
    e?.preventDefault();
    const project = projects.find((p) => p.id === projectId);
    if (!project || adding) return;
    adding = true;
    error = "";
    try {
      const res = await AddFramesToProjectDetailed(project.folder, paths, mode);
      toast.success(`${project.name}: ${describeAddResult(res)}`, {
        action: { label: "Open project", run: () => onopenproject(project) },
      });
      onclose();
    } catch (err) {
      error = String(err);
    } finally {
      adding = false;
    }
  }
</script>

<Modal title="Add to project" {onclose} busy={adding}>
  <p class="desc">{plural(paths.length, "light")} will be added.</p>
  {#if loading}
    <p class="desc">Loading projects…</p>
  {:else if projects.length === 0}
    <p class="desc">There are no projects yet. Use “Create project” instead.</p>
  {:else}
    <form id="add-to-project-form" class="form" onsubmit={submit}>
      <label class="field">
        Project
        <select class="input" bind:value={projectId} data-autofocus>
          {#each projects as p (p.id)}
            <option value={p.id}>{p.name}</option>
          {/each}
        </select>
      </label>
      <fieldset class="mode">
        <legend>Add frames as</legend>
        <label class="mode-opt">
          <input type="radio" name="atpMode" value="symlink" bind:group={mode} />
          Symlinks <span class="hint">(no extra disk space)</span>
        </label>
        <label class="mode-opt">
          <input type="radio" name="atpMode" value="copy" bind:group={mode} />
          Copies
        </label>
      </fieldset>
    </form>
  {/if}
  {#if error}<p class="error" role="alert">{error}</p>{/if}

  {#snippet actions()}
    <button class="btn-ghost" onclick={onclose} disabled={adding}>Cancel</button>
    <button
      class="btn-primary"
      type="submit"
      form="add-to-project-form"
      disabled={adding || projectId === null}
    >
      {adding ? "Adding…" : "Add lights"}
    </button>
  {/snippet}
</Modal>

<style>
  .desc {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .input {
    width: 100%;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-primary);
    font-size: var(--fs-md);
    padding: 6px 10px;
    outline: none;
  }
  .input:focus {
    border-color: var(--accent);
  }
  .mode {
    border: none;
    display: flex;
    flex-wrap: wrap;
    gap: 6px 16px;
    font-size: var(--fs-sm);
    color: var(--text-primary);
  }
  .mode legend {
    width: 100%;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    margin-bottom: 4px;
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
  .hint {
    color: var(--text-secondary);
  }
  .error {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--danger);
  }
</style>
