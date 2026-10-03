<script lang="ts">
  interface Props {
    count: number;
    /** Rejected tab: offer Restore instead of Reject. */
    showRejected: boolean;
    busy?: boolean;
    onreject: () => void;
    onrestore: () => void;
    ondelete: () => void;
    onblink: () => void;
    /** What Blink would show (e.g. "all 240 M 31 lights"); null disables it. */
    blinkLabel: string | null;
    /** Hidden when absent (e.g. server mode). */
    oncreateproject?: () => void;
    onclear: () => void;
  }

  let {
    count,
    showRejected,
    busy = false,
    onreject,
    onrestore,
    ondelete,
    onblink,
    blinkLabel,
    oncreateproject,
    onclear,
  }: Props = $props();
</script>

<div class="selection-bar" role="toolbar" aria-label="Selection actions">
  <span class="sel-count">{count} selected</span>
  {#if showRejected}
    <button class="sel-btn" onclick={onrestore} disabled={busy} title="Restore checked frames (u)">
      Restore {count}
    </button>
  {:else}
    <button
      class="sel-btn sel-reject"
      onclick={onreject}
      disabled={busy}
      title="Reject checked frames (x)"
    >
      Reject {count}
    </button>
  {/if}
  <button
    class="sel-btn sel-danger"
    onclick={ondelete}
    disabled={busy}
    title="Delete checked files from disk (Delete)"
  >
    Delete {count}…
  </button>
  <button
    class="sel-btn"
    onclick={onblink}
    disabled={busy || !blinkLabel}
    title={blinkLabel
      ? `Blink ${blinkLabel} (b)`
      : "No other frames of this object and type in the current view"}
  >
    ▶ Blink
  </button>
  {#if oncreateproject}
    <button class="sel-btn" onclick={oncreateproject} disabled={busy}>Create project…</button>
  {/if}
  <span class="sel-spacer"></span>
  <button class="sel-btn sel-clear" onclick={onclear} title="Clear selection (Esc)">Clear</button>
</div>

<style>
  .selection-bar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    padding: 5px 10px;
    background: color-mix(in srgb, var(--bg-panel) 80%, var(--accent) 20%);
    border-bottom: 1px solid var(--border-accent);
    flex-shrink: 0;
  }

  .sel-count {
    font-size: var(--fs-sm);
    font-weight: 600;
    color: var(--text-primary);
    margin-right: 6px;
  }

  .sel-spacer {
    flex: 1;
  }

  .sel-btn {
    font-size: var(--fs-xs);
    padding: 3px 10px;
    min-height: 26px;
    background: var(--bg-base);
    border: 1px solid var(--border-accent);
    border-radius: 4px;
    color: var(--text-primary);
    cursor: pointer;
    white-space: nowrap;
    transition:
      color 0.12s,
      border-color 0.12s,
      background 0.12s;
  }
  .sel-btn:hover:not(:disabled) {
    border-color: var(--accent);
    color: var(--accent);
  }
  .sel-btn:disabled {
    color: var(--text-dim);
    border-color: var(--border);
    cursor: default;
  }

  .sel-reject:hover:not(:disabled),
  .sel-danger:hover:not(:disabled) {
    border-color: var(--danger);
    color: var(--danger);
  }

  .sel-clear {
    background: transparent;
  }
</style>
