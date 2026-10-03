<script lang="ts">
  import { copyText } from "../../lib/projects/clipboard";

  interface Props {
    path: string;
  }
  let { path }: Props = $props();
</script>

<!-- Truncated at the start (the end of a path is the informative part); click copies. -->
<button class="path-chip" title="{path}&#10;Click to copy" onclick={() => copyText(path)}>
  <span class="path-text">{"‎" + path + "‎"}</span>
  <span class="copy-hint" aria-hidden="true">⧉</span>
</button>

<style>
  .path-chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 100%;
    min-width: 0;
    padding: 2px 6px;
    margin-left: -6px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--text-secondary);
    font-family: monospace;
    font-size: var(--fs-xs);
    cursor: copy;
    text-align: left;
  }
  .path-chip:hover {
    border-color: var(--border);
    color: var(--text-primary);
  }
  .path-text {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    /* rtl puts the ellipsis at the start; the LRM marks keep the path itself LTR. */
    direction: rtl;
    text-align: left;
  }
  .copy-hint {
    flex-shrink: 0;
    color: var(--text-secondary);
  }
</style>
