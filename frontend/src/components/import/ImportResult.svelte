<script lang="ts">
  import type { Progress } from "../../lib/import/session.svelte";

  interface Props {
    progress: Progress;
    /** Retry is offered when the run failed and the source is still known. */
    canRetry: boolean;
    retrying?: boolean;
    onretry: () => void;
    onagain: () => void;
    onviewlibrary?: () => void;
    /** Offered when a run stopped early after copying files (the automatic scan only follows "done"). */
    onscan?: () => void;
  }
  let {
    progress,
    canRetry,
    retrying = false,
    onretry,
    onagain,
    onviewlibrary,
    onscan,
  }: Props = $props();

  let errors = $derived((progress.errors ?? []).map(String));
  let notes = $derived((progress.notes ?? []).map(String));
  let failed = $derived(progress.phase === "error");
  let cancelled = $derived(progress.phase === "cancelled");
</script>

<div class="result">
  <div class="icon" class:ok={!failed && !cancelled} class:bad={failed} aria-hidden="true">
    {failed ? "!" : cancelled ? "■" : "✓"}
  </div>
  <p class="title">
    {failed ? "Import failed" : cancelled ? "Import cancelled" : "Import complete"}
  </p>
  {#if failed && progress.error}
    <p class="error" role="alert">{progress.error}</p>
  {/if}

  <dl class="counts">
    <div>
      <dt>Copied</dt>
      <dd>{progress.copied.toLocaleString()}</dd>
    </div>
    <div>
      <dt>Already in library</dt>
      <dd>{progress.skipped.toLocaleString()}</dd>
    </div>
    {#if progress.kept > 0}
      <div>
        <dt
          title="Delete-from-source was on, but these couldn't be verified identical to the library copy, so they were left on the device"
        >
          Kept on source
        </dt>
        <dd>{progress.kept.toLocaleString()}</dd>
      </div>
    {/if}
    {#if errors.length > 0}
      <div class="err">
        <dt>Errors</dt>
        <dd>{errors.length.toLocaleString()}</dd>
      </div>
    {/if}
  </dl>

  {#if !failed && !cancelled && progress.copied > 0}
    <p class="hint">The library is re-scanned automatically to pick up the new files.</p>
  {:else if progress.copied > 0}
    <p class="hint">
      Files copied before the import stopped aren't in the library index yet.
      {#if onscan}
        <button class="link-btn" onclick={onscan}>Scan library now</button>
      {/if}
    </p>
  {/if}

  {#if errors.length > 0}
    <details class="list err-list">
      <summary>{errors.length} file{errors.length === 1 ? "" : "s"} failed</summary>
      <ul>
        {#each errors as e, i (i)}<li>{e}</li>{/each}
      </ul>
    </details>
  {/if}
  {#if notes.length > 0}
    <details class="list">
      <summary>{notes.length} note{notes.length === 1 ? "" : "s"}</summary>
      <ul>
        {#each notes as n, i (i)}<li>{n}</li>{/each}
      </ul>
    </details>
  {/if}

  <div class="actions">
    {#if failed && canRetry}
      <button class="btn-primary" disabled={retrying} onclick={onretry}>
        {retrying ? "Retrying…" : "Retry"}
      </button>
    {/if}
    {#if onviewlibrary}
      <button class={failed ? "btn-secondary" : "btn-primary"} onclick={onviewlibrary}
        >View in Library</button
      >
    {/if}
    <button class="btn-secondary" onclick={onagain}>Import more</button>
  </div>
</div>

<style>
  .result {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    padding: 40px 24px;
    overflow-y: auto;
    text-align: center;
  }
  .icon {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: var(--fs-xl);
    font-weight: 700;
    border: 2px solid var(--text-secondary);
    color: var(--text-secondary);
    flex-shrink: 0;
  }
  .icon.ok {
    border-color: var(--success);
    color: var(--success);
  }
  .icon.bad {
    border-color: var(--danger);
    color: var(--danger);
  }
  .title {
    margin: 0;
    font-size: var(--fs-xl);
    font-weight: 600;
    color: var(--text-primary);
  }
  .error {
    margin: 0;
    max-width: 560px;
    color: var(--danger);
    font-size: var(--fs-md);
    word-break: break-word;
  }
  .counts {
    display: flex;
    gap: 24px;
    margin: 6px 0 0;
    flex-wrap: wrap;
    justify-content: center;
  }
  .counts div {
    display: flex;
    flex-direction: column-reverse;
    align-items: center;
    gap: 2px;
  }
  .counts dt {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .counts dd {
    margin: 0;
    font-size: var(--fs-xl);
    font-weight: 600;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  .counts .err dd {
    color: var(--danger);
  }
  .hint {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .link-btn {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: var(--accent);
    text-decoration: underline;
    cursor: pointer;
  }
  .list {
    width: min(640px, 100%);
    text-align: left;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 6px 12px;
    font-size: var(--fs-sm);
  }
  .list summary {
    cursor: pointer;
    color: var(--text-primary);
    padding: 2px 0;
  }
  .err-list summary {
    color: var(--danger);
  }
  .list ul {
    margin: 6px 0 4px;
    padding-left: 18px;
    max-height: 240px;
    overflow-y: auto;
    font-family: monospace;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    display: flex;
    flex-direction: column;
    gap: 3px;
    word-break: break-all;
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-top: 12px;
  }
</style>
