<script lang="ts">
  import type { FrameInfo } from "../../lib/slideshow/frameInfo";

  interface Props {
    info: FrameInfo;
  }

  let { info }: Props = $props();
</script>

<div class="card">
  <p class="title">{info.title}</p>
  {#if info.rows.length > 0}
    <dl>
      {#each info.rows as [label, value] (label)}
        <dt>{label}</dt>
        <dd>{value}</dd>
      {/each}
    </dl>
  {/if}
</div>

<style>
  .card {
    position: absolute;
    top: 16px;
    left: 16px;
    max-width: min(320px, calc(100% - 32px));
    padding: 10px 14px 11px;
    background: rgb(0 0 0 / 0.55);
    border: 1px solid rgb(255 255 255 / 0.14);
    border-radius: 8px;
    backdrop-filter: blur(6px);
    color: #e8e8ea;
    pointer-events: none;
  }

  .title {
    font-size: var(--fs-lg);
    font-weight: 600;
    line-height: 1.25;
    overflow-wrap: anywhere;
  }

  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 3px 12px;
    margin: 8px 0 0;
    font-size: var(--fs-sm);
  }
  dt {
    color: #a8a8b0;
  }
  dd {
    margin: 0;
    font-variant-numeric: tabular-nums;
  }
</style>
