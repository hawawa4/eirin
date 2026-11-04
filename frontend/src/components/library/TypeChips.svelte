<script lang="ts">
  import type { FrameType } from "../../lib/types";
  import { frameTypeMeta } from "../../lib/library/groups";

  interface Props {
    /** Frames of each type that pass the other filters, in display order. */
    counts: { type: string; count: number }[];
    /** Frames passing the other filters (the "All" chip). */
    total: number;
    selected: readonly FrameType[];
    /** `additive` (Ctrl/Shift-click) adds/removes one type instead of showing only it. */
    onpick: (type: FrameType | null, additive: boolean) => void;
  }

  let { counts, total, selected, onpick }: Props = $props();
</script>

<div class="type-chips" role="group" aria-label="Show frame types">
  <button
    class="chip"
    class:active={selected.length === 0}
    aria-pressed={selected.length === 0}
    onclick={() => onpick(null, false)}
    title="Show every frame type"
  >
    <span class="chip-label">All</span>
    <span class="chip-count">{total.toLocaleString()}</span>
  </button>
  {#each counts as c (c.type)}
    {@const meta = frameTypeMeta(c.type)}
    {@const on = selected.includes(c.type as FrameType)}
    <button
      class="chip"
      class:active={on}
      class:empty={c.count === 0 && !on}
      style:--chip-color={meta.color}
      aria-pressed={on}
      onclick={(e) => onpick(c.type as FrameType, e.ctrlKey || e.metaKey || e.shiftKey)}
      title="Show only {meta.label.toLowerCase()} frames · Ctrl+click to combine types"
    >
      <span class="chip-dot" aria-hidden="true"></span>
      <span class="chip-label">{meta.label}</span>
      <span class="chip-count">{c.count.toLocaleString()}</span>
    </button>
  {/each}
</div>

<style>
  .type-chips {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px;
  }

  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 3px 10px;
    min-height: 26px;
    font-size: var(--fs-xs);
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 13px;
    color: var(--text-secondary);
    cursor: pointer;
    white-space: nowrap;
    transition:
      background 0.12s,
      border-color 0.12s,
      color 0.12s;
  }
  .chip:hover {
    color: var(--text-primary);
    border-color: var(--chip-color, var(--accent));
  }
  .chip.active {
    color: var(--text-primary);
    border-color: var(--chip-color, var(--accent));
    background: color-mix(in srgb, var(--bg-base) 72%, var(--chip-color, var(--accent)) 28%);
  }
  .chip.empty {
    opacity: 0.5;
  }

  .chip-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--chip-color);
  }
  .chip-label {
    font-weight: 600;
  }
  .chip-count {
    font-variant-numeric: tabular-nums;
    color: var(--text-secondary);
  }
  .chip.active .chip-count {
    color: var(--text-primary);
  }
</style>
