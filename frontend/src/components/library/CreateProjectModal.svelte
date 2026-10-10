<script lang="ts">
  import { AddFramesToProjectDetailed, CreateProject } from "$app";
  import type { Project } from "../../lib/types";
  import { toast } from "../../lib/toast.svelte";
  import {
    describeTypeCounts,
    isProjectFrameType,
    type TypedFrame,
  } from "../../lib/projects/frames";
  import { plural } from "../../lib/utils";
  import Modal from "../Modal.svelte";

  interface Props {
    frames: TypedFrame[];
    /** Prefills the name field. */
    suggestedName?: string;
    onclose: () => void;
    oncreated: (project: Project) => void;
  }

  let { frames, suggestedName = "", onclose, oncreated }: Props = $props();

  // svelte-ignore state_referenced_locally
  let name = $state(suggestedName);
  let mode = $state<"symlink" | "copy">("symlink");
  let error = $state("");
  let creating = $state(false);

  let eligible = $derived(frames.filter((f) => isProjectFrameType(f.frameType)));
  let ineligibleCount = $derived(frames.length - eligible.length);

  async function submit(e?: Event) {
    e?.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || creating) return;
    creating = true;
    error = "";
    let project: Project;
    try {
      project = await CreateProject(trimmed, "");
    } catch (err) {
      error = String(err);
      creating = false;
      return;
    }
    try {
      const paths = eligible.map((f) => f.nasPath);
      const res =
        paths.length > 0
          ? await AddFramesToProjectDetailed(project.folder, paths, mode)
          : { added: 0, alreadyPresent: 0, skipped: [] };
      const skipped = (res.skipped?.length ?? 0) + ineligibleCount;
      const parts = [`${plural(res.added, "frame")} added`];
      if (res.alreadyPresent) parts.push(`${res.alreadyPresent} already present`);
      if (skipped) parts.push(`${skipped} skipped (not light/dark/flat/bias)`);
      toast.success(`Created project “${project.name}”: ${parts.join(", ")}`);
    } catch (err) {
      // The project exists; only adding frames failed. Still take the user there.
      toast.error(`Project “${project.name}” created, but adding frames failed: ${String(err)}`);
    } finally {
      creating = false;
    }
    oncreated(project);
    onclose();
  }
</script>

<Modal title="Create project" {onclose} busy={creating}>
  <p class="desc">
    {plural(eligible.length, "frame")} will be added{eligible.length
      ? `: ${describeTypeCounts(eligible)}`
      : ""}.
    {#if ineligibleCount > 0}
      <span class="skipped">{ineligibleCount} of another type will be skipped.</span>
    {/if}
  </p>
  <form id="create-project-form" class="form" onsubmit={submit}>
    <label class="field">
      Project name
      <input
        class="input"
        type="text"
        placeholder="e.g. M31 autumn 2026"
        bind:value={name}
        spellcheck="false"
        data-autofocus
      />
    </label>
    <fieldset class="mode">
      <legend>Add frames as</legend>
      <label class="mode-opt">
        <input type="radio" name="cpMode" value="symlink" bind:group={mode} />
        Symlinks <span class="hint">(no extra disk space)</span>
      </label>
      <label class="mode-opt">
        <input type="radio" name="cpMode" value="copy" bind:group={mode} />
        Copies
      </label>
    </fieldset>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </form>

  {#snippet actions()}
    <button class="btn-ghost" onclick={onclose} disabled={creating}>Cancel</button>
    <button
      class="btn-primary"
      type="submit"
      form="create-project-form"
      disabled={creating || !name.trim()}
    >
      {creating ? "Creating…" : "Create project"}
    </button>
  {/snippet}
</Modal>

<style>
  .desc {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .skipped {
    color: var(--text-secondary);
    font-style: italic;
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
  .input::placeholder {
    color: var(--text-dim);
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
