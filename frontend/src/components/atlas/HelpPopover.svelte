<script lang="ts">
  import { ATLAS_COLORS } from "../../lib/atlas/draw";

  interface Props {
    open: boolean;
  }

  let { open = $bindable() }: Props = $props();

  let root = $state<HTMLElement | null>(null);

  function onWindowMousedown(e: MouseEvent) {
    if (open && root && e.target instanceof Node && !root.contains(e.target)) open = false;
  }

  const CONTROLS: [string, string][] = [
    ["Scroll", "Zoom at pointer"],
    ["Drag", "Pan"],
    ["Click frame", "Preview it and show details"],
    ["Shift/Ctrl + click", "Add to / remove from overlay"],
    ["Click empty sky", "Clear selection"],
    ["+  /  −", "Zoom in / out"],
    ["0", "Fit all frames"],
    ["Arrow keys", "Pan"],
    ["/", "Search objects"],
    ["Esc", "Clear selection"],
  ];

  const LEGEND: { color: string; label: string; shape: "box" | "dot" | "ring" }[] = [
    { color: ATLAS_COLORS.frame, label: "Your frames", shape: "box" },
    { color: ATLAS_COLORS.selected, label: "Selected / overlaid", shape: "box" },
    { color: ATLAS_COLORS.hovered, label: "Hovered", shape: "box" },
    { color: ATLAS_COLORS.star, label: "Catalog stars", shape: "dot" },
    { color: ATLAS_COLORS.dso, label: "Deep-sky objects", shape: "ring" },
  ];
</script>

<svelte:window onmousedown={onWindowMousedown} />

<div class="help" bind:this={root}>
  <button
    class="help-btn"
    class:active={open}
    aria-expanded={open}
    aria-haspopup="dialog"
    title="Atlas help"
    onclick={() => (open = !open)}>?</button
  >
  {#if open}
    <div class="help-pop" role="dialog" aria-label="Sky Atlas help">
      <h3>Controls</h3>
      <dl>
        {#each CONTROLS as [k, v] (k)}
          <dt><kbd>{k}</kbd></dt>
          <dd>{v}</dd>
        {/each}
      </dl>
      <h3>Legend</h3>
      <ul>
        {#each LEGEND as l (l.label)}
          <li>
            <span class="swatch {l.shape}" style="--c: {l.color}"></span>
            {l.label}
          </li>
        {/each}
      </ul>
      <p class="note">
        North is up and East is left, as the sky looks from the ground. Frames need WCS coordinates
        (plate-solved) to appear here.
      </p>
    </div>
  {/if}
</div>

<style>
  .help {
    position: relative;
  }
  .help-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: color-mix(in srgb, var(--bg-panel) 92%, transparent);
    border: 1px solid var(--border);
    border-radius: 50%;
    color: var(--text-secondary);
    font-size: var(--fs-md);
    font-weight: 700;
    cursor: pointer;
  }
  .help-btn:hover {
    color: var(--text-primary);
    border-color: var(--border-accent);
  }
  .help-btn.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }
  .help-pop {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    width: 300px;
    max-height: 70vh;
    overflow-y: auto;
    padding: 12px 14px;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 7px;
    box-shadow: 0 8px 28px rgba(0, 0, 0, 0.5);
    z-index: 50;
    font-size: var(--fs-sm);
    color: var(--text-primary);
  }
  h3 {
    margin: 0 0 6px;
    font-size: var(--fs-xs);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
  }
  h3:not(:first-child) {
    margin-top: 12px;
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 4px 10px;
    margin: 0;
    align-items: baseline;
  }
  dt {
    white-space: nowrap;
  }
  dd {
    margin: 0;
    color: var(--text-secondary);
  }
  kbd {
    font-family: inherit;
    font-size: var(--fs-xs);
    padding: 1px 5px;
    border: 1px solid var(--border-accent);
    border-radius: 3px;
    background: var(--bg-row);
    color: var(--text-primary);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  li {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .swatch {
    width: 14px;
    height: 10px;
    flex-shrink: 0;
  }
  .swatch.box {
    border: 1.5px solid var(--c);
    background: color-mix(in srgb, var(--c) 20%, transparent);
    border-radius: 1px;
  }
  .swatch.dot {
    width: 8px;
    height: 8px;
    margin: 0 3px;
    border-radius: 50%;
    background: var(--c);
  }
  .swatch.ring {
    width: 10px;
    height: 10px;
    margin: 0 2px;
    border-radius: 50%;
    border: 1.5px solid var(--c);
  }
  .note {
    margin: 12px 0 0;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    line-height: 1.45;
  }
</style>
