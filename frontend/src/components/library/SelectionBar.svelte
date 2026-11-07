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
    /** Processed/image files in the selection; the button is hidden when 0. */
    publishable?: number;
    /** All of those are already public: the button removes them instead. */
    allPublic?: boolean;
    onsetpublic?: (value: boolean) => void;
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
    publishable = 0,
    allPublic = false,
    onsetpublic,
  }: Props = $props();
</script>

<div class="selection-bar" role="toolbar" aria-label="Selection actions">
  <span class="sel-count">{count.toLocaleString()} selected</span>
  {#if oncreateproject && !showRejected}
    <button
      class="sel-btn sel-primary"
      onclick={oncreateproject}
      disabled={busy}
      title="New Siril project from the selected light, dark, flat and bias frames"
      >＋ New project…</button
    >
  {/if}
  <button
    class="sel-btn"
    onclick={onblink}
    disabled={busy || !blinkLabel}
    title={blinkLabel
      ? `Blink ${blinkLabel}`
      : "No other frames of this object and type in the current view"}
  >
    Blink <kbd>b</kbd>
  </button>
  {#if showRejected}
    <button class="sel-btn" onclick={onrestore} disabled={busy} title="Restore the selected frames">
      Restore <kbd>u</kbd>
    </button>
  {:else}
    <button
      class="sel-btn sel-reject"
      onclick={onreject}
      disabled={busy}
      title="Reject the selected frames — they move to the Rejected tab and stay on disk"
    >
      Reject <kbd>x</kbd>
    </button>
  {/if}
  {#if onsetpublic && publishable > 0 && !showRejected}
    <button
      class="sel-btn"
      class:sel-public={!allPublic}
      onclick={() => onsetpublic(!allPublic)}
      disabled={busy}
      title={allPublic
        ? "Remove the selected images from the public gallery"
        : "Show the selected processed images in the public gallery"}
    >
      {allPublic ? "☆ Unpublish" : "★ Public"} <kbd>p</kbd>
    </button>
  {/if}
  <span class="sel-spacer"></span>
  <button
    class="sel-btn sel-danger"
    onclick={ondelete}
    disabled={busy}
    title="Delete the selected files from disk (asks first)"
  >
    Delete from disk… <kbd>Del</kbd>
  </button>
  <button class="sel-btn sel-clear" onclick={onclear} title="Clear the selection">
    Clear <kbd>Esc</kbd>
  </button>
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

  .sel-public:hover:not(:disabled) {
    border-color: var(--accent);
    color: var(--accent);
  }

  .sel-clear {
    background: transparent;
  }

  .sel-primary {
    border-color: var(--accent);
    color: var(--accent);
  }
  .sel-primary:hover:not(:disabled) {
    background: var(--accent);
    color: var(--accent-contrast);
  }

  kbd {
    margin-left: 4px;
    font-family: inherit;
    font-size: 0.6875rem;
    color: var(--text-dim);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 0 4px;
  }
</style>
