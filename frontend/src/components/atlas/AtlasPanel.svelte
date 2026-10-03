<script lang="ts">
  import type * as app from "$models/app";
  import { formatDec, formatRA } from "../../lib/utils";
  import type { FrameSize } from "../../lib/atlas/footprint";
  import type { OrientationFix } from "../../lib/atlas/orientation";

  interface Props {
    /** Frame shown in the panel (top of the overlay). */
    entry: app.AtlasIndexEntry;
    size: FrameSize | undefined;
    /** All overlaid frames, z-order (last = top = `entry`). */
    overlay: app.AtlasIndexEntry[];
    previewLoading: boolean;
    fix: OrientationFix;
    onclose: () => void;
    onbringtofront: (path: string) => void;
    onremove: (path: string) => void;
    onclearall: () => void;
    onfixchange: (fix: OrientationFix) => void;
    onopen?: (nasPath: string) => void;
  }

  let {
    entry,
    size,
    overlay,
    previewLoading,
    fix,
    onclose,
    onbringtofront,
    onremove,
    onclearall,
    onfixchange,
    onopen,
  }: Props = $props();

  // Top of the list = top of the stack.
  let stack = $derived([...overlay].reverse());
  let fixed = $derived(fix.offset !== 0 || fix.mirror);

  const OFFSETS: { value: number; label: string; title: string }[] = [
    { value: -90, label: "↺ 90°", title: "Rotate 90° counter-clockwise" },
    { value: 0, label: "0°", title: "No adjustment" },
    { value: 90, label: "↻ 90°", title: "Rotate 90° clockwise" },
    { value: 180, label: "180°", title: "Rotate 180°" },
  ];

  function fov(px: number): string {
    const deg = (px * entry.pixelScale) / 3600;
    return deg < 1 ? `${(deg * 60).toFixed(1)}′` : `${deg.toFixed(2)}°`;
  }
</script>

<aside class="atlas-panel" aria-label="Selected frame">
  <div class="ap-header">
    <div class="ap-title" title={entry.object || entry.name}>{entry.object || entry.name}</div>
    <button
      class="ap-close"
      onclick={onclose}
      title="Close this frame"
      aria-label="Close this frame">✕</button
    >
  </div>

  <div class="ap-body">
    <dl class="ap-grid">
      <dt>Type</dt>
      <dd>{entry.frameType}{previewLoading ? " · loading preview…" : ""}</dd>
      <dt>RA</dt>
      <dd title="{entry.ra.toFixed(5)}°">{formatRA(entry.ra)}</dd>
      <dt>Dec</dt>
      <dd title="{entry.dec.toFixed(5)}°">{formatDec(entry.dec)}</dd>
      <dt>Scale</dt>
      <dd>{entry.pixelScale.toFixed(2)} ″/px</dd>
      <dt>Rotation</dt>
      <dd>{entry.rotation.toFixed(1)}°</dd>
      {#if size}
        <dt>Size</dt>
        <dd>{size.width} × {size.height} px</dd>
        <dt>Field</dt>
        <dd>{fov(size.width)} × {fov(size.height)}</dd>
      {/if}
      <dt>File</dt>
      <dd class="ap-file" title={entry.nasPath}>{entry.name}</dd>
    </dl>

    {#if onopen}
      <button class="ap-open-btn" onclick={() => onopen(entry.nasPath)}>Open in Library →</button>
    {/if}

    {#if overlay.length > 1}
      <section class="ap-overlay">
        <div class="ap-overlay-head">
          <span>Overlaid frames ({overlay.length})</span>
          <button class="ap-link" onclick={onclearall}>Clear all</button>
        </div>
        <ul>
          {#each stack as f, i (f.nasPath)}
            <li class:top={i === 0}>
              <button
                class="ap-ov-name"
                onclick={() => onbringtofront(f.nasPath)}
                title={i === 0 ? `${f.name} (on top)` : `Bring ${f.name} to front`}
              >
                {f.object || f.name}
                <span class="ap-ov-file">{f.name}</span>
              </button>
              <button
                class="ap-ov-remove"
                onclick={() => onremove(f.nasPath)}
                title="Remove from overlay"
                aria-label="Remove {f.name} from overlay">✕</button
              >
            </li>
          {/each}
        </ul>
      </section>
    {/if}

    <details class="ap-fix">
      <summary>Fix orientation{fixed ? " (adjusted)" : ""}</summary>
      <p class="ap-fix-hint">
        Only needed if the preview doesn't line up with the stars around it (e.g. a wrong rotation
        sign from the plate solver). Applies for this session.
      </p>
      <div class="ap-fix-btns" role="group" aria-label="Rotation adjustment">
        {#each OFFSETS as o (o.value)}
          <button
            class="fix-btn"
            class:active={fix.offset === o.value}
            aria-pressed={fix.offset === o.value}
            title={o.title}
            onclick={() => onfixchange({ ...fix, offset: o.value })}>{o.label}</button
          >
        {/each}
      </div>
      <label class="ap-fix-mirror">
        <input
          type="checkbox"
          checked={fix.mirror}
          onchange={(e) => onfixchange({ ...fix, mirror: e.currentTarget.checked })}
        />
        Invert rotation angle
      </label>
    </details>
  </div>
</aside>

<style>
  .atlas-panel {
    position: absolute;
    top: 48px;
    right: 10px;
    width: 270px;
    background: color-mix(in srgb, var(--bg-panel) 96%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 7px;
    z-index: 30;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    max-height: calc(100% - 184px);
    backdrop-filter: blur(4px);
    box-shadow: 0 6px 24px rgba(0, 0, 0, 0.4);
  }
  .ap-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 8px 8px 12px;
    border-bottom: 1px solid var(--border);
    gap: 6px;
    flex-shrink: 0;
  }
  .ap-title {
    font-size: var(--fs-md);
    font-weight: 600;
    color: var(--text-primary);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ap-close {
    flex-shrink: 0;
    width: 26px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: 1px solid transparent;
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: var(--fs-sm);
  }
  .ap-close:hover {
    color: var(--text-primary);
    border-color: var(--border);
    background: var(--bg-row-hover);
  }

  .ap-body {
    padding: 10px 12px 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    overflow-y: auto;
    min-height: 0;
  }
  .ap-grid {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 3px 10px;
    margin: 0;
    font-size: var(--fs-sm);
  }
  .ap-grid dt {
    color: var(--text-secondary);
  }
  .ap-grid dd {
    margin: 0;
    color: var(--text-primary);
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .ap-file {
    font-size: var(--fs-xs);
    word-break: break-all;
    color: var(--text-secondary);
  }

  .ap-open-btn {
    padding: 7px 0;
    background: var(--accent);
    border: 1px solid var(--accent);
    border-radius: 5px;
    color: var(--accent-contrast);
    font-size: var(--fs-sm);
    font-weight: 600;
    cursor: pointer;
  }
  .ap-open-btn:hover {
    filter: brightness(1.1);
  }

  .ap-overlay {
    border-top: 1px solid var(--border);
    padding-top: 8px;
  }
  .ap-overlay-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    font-size: var(--fs-xs);
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 4px;
  }
  .ap-link {
    background: none;
    border: none;
    padding: 0;
    color: var(--accent);
    font-size: var(--fs-xs);
    cursor: pointer;
  }
  .ap-link:hover {
    text-decoration: underline;
  }
  .ap-overlay ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .ap-overlay li {
    display: flex;
    align-items: stretch;
    border-radius: 4px;
    border-left: 3px solid transparent;
  }
  .ap-overlay li.top {
    border-left-color: var(--atlas-selected);
    background: var(--bg-row);
  }
  .ap-ov-name {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    background: none;
    border: none;
    padding: 3px 6px;
    cursor: pointer;
    color: var(--text-primary);
    font-size: var(--fs-sm);
    text-align: left;
  }
  .ap-ov-file {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ap-ov-name:hover {
    background: var(--bg-row-hover);
  }
  .ap-ov-remove {
    flex-shrink: 0;
    width: 26px;
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: var(--fs-xs);
  }
  .ap-ov-remove:hover {
    color: var(--danger);
  }

  .ap-fix {
    border-top: 1px solid var(--border);
    padding-top: 8px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .ap-fix summary {
    cursor: pointer;
    font-weight: 600;
  }
  .ap-fix summary:hover {
    color: var(--text-primary);
  }
  .ap-fix-hint {
    margin: 6px 0;
    line-height: 1.4;
  }
  .ap-fix-btns {
    display: flex;
    gap: 4px;
  }
  .fix-btn {
    flex: 1;
    padding: 3px 0;
    font-size: var(--fs-xs);
    background: var(--bg-row);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
  }
  .fix-btn:hover {
    border-color: var(--border-accent);
    color: var(--text-primary);
  }
  .fix-btn.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }
  .ap-fix-mirror {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 6px;
    cursor: pointer;
  }
</style>
