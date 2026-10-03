<script lang="ts" module>
  export interface FrameMetaEdit {
    object: string;
    telescope: string;
    filter: string;
    dateObs: string;
  }
</script>

<script lang="ts">
  import { UpdateFrameMeta } from "$app";
  import Modal from "../Modal.svelte";

  interface Props {
    path: string;
    initial: FrameMetaEdit;
    onclose: () => void;
    onsaved: (path: string, meta: FrameMetaEdit) => void;
  }

  let { path, initial, onclose, onsaved }: Props = $props();

  // Seeded once from the initial prop; the fields are then user-edited.
  let meta = $state<FrameMetaEdit>((() => ({ ...initial }))());
  let error = $state("");
  let working = $state(false);

  async function submit(e?: Event) {
    e?.preventDefault();
    if (working) return;
    working = true;
    error = "";
    try {
      await UpdateFrameMeta(path, {
        Object: meta.object,
        Telescope: meta.telescope,
        Filter: meta.filter,
        DateObs: meta.dateObs,
      });
      onsaved(path, { ...meta });
      onclose();
    } catch (err) {
      error = String(err);
    } finally {
      working = false;
    }
  }
</script>

<Modal title="Edit metadata" {onclose} busy={working}>
  <p class="desc">Overrides the catalog metadata. Empty fields are left unchanged.</p>
  <form id="meta-form" class="form" onsubmit={submit}>
    <label class="field">
      Object
      <input class="input" type="text" bind:value={meta.object} spellcheck="false" data-autofocus />
    </label>
    <label class="field">
      Telescope
      <input class="input" type="text" bind:value={meta.telescope} spellcheck="false" />
    </label>
    <label class="field">
      Filter
      <input class="input" type="text" bind:value={meta.filter} spellcheck="false" />
    </label>
    <label class="field">
      Date (ISO)
      <input
        class="input"
        type="text"
        bind:value={meta.dateObs}
        placeholder="2024-01-15T22:30:00"
        spellcheck="false"
      />
    </label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </form>

  {#snippet actions()}
    <button class="btn-ghost" onclick={onclose} disabled={working}>Cancel</button>
    <button class="btn-primary" type="submit" form="meta-form" disabled={working}>
      {working ? "Saving…" : "Save"}
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
    gap: 10px;
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
    font-family: monospace;
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
  .error {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--danger);
  }
</style>
