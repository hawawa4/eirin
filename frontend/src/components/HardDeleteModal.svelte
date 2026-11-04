<script lang="ts">
  import Modal from "./Modal.svelte";
  import { formatBytes } from "../lib/utils";

  interface Props {
    /** `count` > 1 words it for several files; `bytes` adds the space freed. */
    target: { name: string; count?: number; bytes?: number };
    onconfirm: () => void;
    oncancel: () => void;
  }

  let { target, onconfirm, oncancel }: Props = $props();

  let count = $derived(target.count ?? 1);
  let many = $derived(count > 1);
</script>

<Modal
  title={many ? `Delete ${count.toLocaleString()} files permanently?` : "Delete file permanently?"}
  onclose={oncancel}
  danger
  width="min(420px, 92vw)"
>
  <p class="hd-filename">{target.name}</p>
  <p class="hd-sub">
    This cannot be undone. {many ? "The files" : "The file"} will be removed from disk{target.bytes
      ? `, freeing ${formatBytes(target.bytes)}`
      : ""}.
  </p>

  {#snippet actions()}
    <button class="btn-ghost" data-autofocus onclick={oncancel}>Cancel</button>
    <button class="btn-danger" onclick={onconfirm}
      >{many ? `Delete ${count.toLocaleString()} files` : "Delete forever"}</button
    >
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
