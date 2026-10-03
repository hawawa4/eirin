<script lang="ts">
  import type * as app from "$models/app";
  import type { ColumnDef } from "../../lib/types";
  import { FRAME_TYPE_META } from "../../lib/types";
  import { getLibraryCellValue } from "../../lib/utils";
  import { frameTypeMeta } from "../../lib/library/groups";

  interface Props {
    frame: app.LibraryFrame;
    columns: ColumnDef[];
    /** Row is the preview cursor. */
    current: boolean;
    /** Row's checkbox is ticked (multi-selection). */
    checked: boolean;
    onrowclick: (e: MouseEvent) => void;
    onctxmenu: (e: MouseEvent) => void;
    oncheck: () => void;
    onchangetype: (newType: string, select: HTMLSelectElement) => void;
  }

  let { frame, columns, current, checked, onrowclick, onctxmenu, oncheck, onchangetype }: Props =
    $props();

  let meta = $derived(frameTypeMeta(frame.frameType));
</script>

<tr
  class="frame-row"
  class:current
  class:checked
  data-path={frame.nasPath}
  aria-selected={current}
  onclick={onrowclick}
  oncontextmenu={onctxmenu}
>
  <td
    class="cb-td"
    onclick={(e) => {
      e.stopPropagation();
      oncheck();
    }}
  >
    <input
      type="checkbox"
      {checked}
      aria-label="Select {frame.fileName}"
      onclick={(e) => e.stopPropagation()}
      onchange={oncheck}
    />
  </td>
  {#each columns as col (col.id)}
    <td class="col-{col.id}">
      {#if col.id === "frameType"}
        <select
          class="type-select"
          value={frame.frameType}
          style="color:{meta.color};background:{meta.bg}"
          aria-label="Frame type"
          onclick={(e) => e.stopPropagation()}
          onchange={(e) => {
            const el = e.currentTarget as HTMLSelectElement;
            el.blur(); // hand the keyboard back to the culling shortcuts
            onchangetype(el.value, el);
          }}
        >
          {#each Object.entries(FRAME_TYPE_META) as [val, m] (val)}
            <option value={val}>{m.short}</option>
          {/each}
        </select>
      {:else if col.id === "name"}
        <span class="file-name">{frame.fileName}</span>
      {:else}
        {getLibraryCellValue(frame, col.id)}
      {/if}
    </td>
  {/each}
</tr>

<style>
  .frame-row {
    cursor: pointer;
  }

  .frame-row td {
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .frame-row:hover td {
    background: var(--bg-row-hover);
  }

  .frame-row.checked td {
    background: color-mix(in srgb, var(--accent-dim) 55%, var(--bg-base) 45%);
  }

  .frame-row.current td {
    background: var(--accent-dim);
  }
  .frame-row.current td:first-child {
    box-shadow: inset 3px 0 0 var(--accent);
  }

  .cb-td {
    padding: 0 !important;
    text-align: center;
    cursor: default;
    width: 28px;
  }

  .cb-td input[type="checkbox"] {
    accent-color: var(--accent);
    cursor: pointer;
    width: 14px;
    height: 14px;
    vertical-align: middle;
  }

  .col-frameType {
    padding: 4px 6px !important;
  }

  .col-expTime,
  .col-size,
  .col-gain,
  .col-ccdTemp,
  .col-fwhm,
  .col-starCount,
  .col-background,
  .col-noise,
  .col-snr {
    text-align: right;
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }

  .col-expTime,
  .col-size,
  .col-gain,
  .col-ccdTemp,
  .col-dateObs {
    color: var(--text-secondary) !important;
  }

  .col-dateObs {
    font-size: var(--fs-sm);
    font-variant-numeric: tabular-nums;
  }

  .file-name {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .type-select {
    appearance: none;
    -webkit-appearance: none;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 3px;
    font-size: var(--fs-xs);
    font-weight: 600;
    font-family: "Consolas", "Fira Code", monospace;
    padding: 2px 5px;
    cursor: pointer;
    width: 100%;
    text-align: center;
    outline: none;
    transition: border-color 0.12s;
  }

  .type-select:hover {
    border-color: rgba(255, 255, 255, 0.3);
  }

  .type-select option {
    background: var(--bg-panel);
    color: var(--text-primary);
    font-weight: normal;
  }
</style>
