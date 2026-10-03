<script lang="ts">
  import Modal from "../Modal.svelte";

  interface Props {
    count: number;
    sourceFolder: string;
    onconfirm: () => void;
    oncancel: () => void;
  }
  let { count, sourceFolder, onconfirm, oncancel }: Props = $props();
</script>

<Modal title="Delete files from the source?" onclose={oncancel} danger width="min(500px, 92vw)">
  <p>
    Eirin will copy {count.toLocaleString()}
    {count === 1 ? "file" : "files"} into your library, then delete them from
  </p>
  <p class="path">{sourceFolder}</p>
  <ul>
    <li>
      A source file is deleted <strong>only after</strong> its library copy is verified identical (same
      size and full SHA-256 checksum).
    </li>
    <li>
      This includes files skipped because they're already in the library: their source copy is
      deleted once the existing library copy is verified identical.
    </li>
    <li>
      Files that can't be verified are kept on the device and listed when the import finishes.
    </li>
  </ul>

  {#snippet actions()}
    <button class="btn-ghost" data-autofocus onclick={oncancel}>Cancel</button>
    <button class="btn-danger" onclick={onconfirm}>Import &amp; delete</button>
  {/snippet}
</Modal>

<style>
  p {
    margin: 0;
    color: var(--text-primary);
  }
  .path {
    font-family: monospace;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    word-break: break-all;
  }
  ul {
    margin: 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  strong {
    color: var(--text-primary);
  }
</style>
