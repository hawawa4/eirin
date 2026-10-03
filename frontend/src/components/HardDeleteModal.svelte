<script lang="ts">
  import Modal from "./Modal.svelte";

  interface Props {
    target: { name: string };
    onconfirm: () => void;
    oncancel: () => void;
  }

  let { target, onconfirm, oncancel }: Props = $props();
</script>

<Modal title="Delete file permanently?" onclose={oncancel} danger width="min(400px, 92vw)">
  <p class="hd-filename">{target.name}</p>
  <p class="hd-sub">This cannot be undone. The file will be removed from disk.</p>

  {#snippet actions()}
    <button class="btn-ghost" data-autofocus onclick={oncancel}>Cancel</button>
    <button class="btn-danger" onclick={onconfirm}>Delete Forever</button>
  {/snippet}
</Modal>

<style>
  .hd-filename {
    margin: 0;
    font-family: "Consolas", "Fira Code", monospace;
    font-size: var(--fs-sm);
    color: var(--accent);
    word-break: break-all;
    user-select: text;
  }
  .hd-sub {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
</style>
