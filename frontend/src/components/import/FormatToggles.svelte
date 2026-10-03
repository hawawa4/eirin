<script lang="ts">
  import { FORMAT_GROUPS, selectedFormats, toggleFormat } from "../../lib/import/session.svelte";

  interface Props {
    label?: string;
    disabled?: boolean;
  }
  let { label = "Formats:", disabled = false }: Props = $props();
</script>

<div class="formats" role="group" aria-label="File formats to import">
  <span class="formats-label">{label}</span>
  {#each FORMAT_GROUPS as g (g.key)}
    <button
      class="tool-btn"
      class:active={selectedFormats.has(g.key)}
      aria-pressed={selectedFormats.has(g.key)}
      {disabled}
      onclick={() => toggleFormat(g.key)}>{g.label}</button
    >
  {/each}
</div>

<style>
  .formats {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }
  .formats-label {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .tool-btn {
    font-size: var(--fs-sm);
    font-weight: 600;
  }
  .tool-btn:disabled {
    cursor: default;
  }
</style>
