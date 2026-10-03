<script lang="ts">
  import { RenameFrame } from "$app";
  import Modal from "../Modal.svelte";

  interface Props {
    path: string;
    name: string;
    onclose: () => void;
    onrenamed: (oldPath: string, newPath: string, newName: string) => void;
  }

  let { path, name, onclose, onrenamed }: Props = $props();

  // Seeded once from the initial prop; the field is then user-edited.
  let value = $state((() => name)());
  let error = $state("");
  let working = $state(false);

  async function submit(e?: Event) {
    e?.preventDefault();
    const next = value.trim();
    if (!next || working) return;
    if (next === name) {
      onclose();
      return;
    }
    working = true;
    error = "";
    try {
      const newPath = await RenameFrame(path, next);
      onrenamed(path, newPath, next);
      onclose();
    } catch (err) {
      error = String(err);
    } finally {
      working = false;
    }
  }
</script>

<Modal title="Rename file" {onclose} busy={working}>
  <form id="rename-form" class="form" onsubmit={submit}>
    <label class="field">
      New filename (same folder)
      <input
        class="input"
        type="text"
        bind:value
        spellcheck="false"
        data-autofocus
        onfocus={(e) => {
          const el = e.currentTarget;
          const dot = el.value.lastIndexOf(".");
          el.setSelectionRange(0, dot > 0 ? dot : el.value.length);
        }}
      />
    </label>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </form>

  {#snippet actions()}
    <button class="btn-ghost" onclick={onclose} disabled={working}>Cancel</button>
    <button
      class="btn-primary"
      type="submit"
      form="rename-form"
      disabled={working || !value.trim()}
    >
      {working ? "Renaming…" : "Rename"}
    </button>
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
  .error {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--danger);
  }
</style>
