<script lang="ts">
  import { onMount } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import type * as app from "$models/app";
  import { SuggestRejects } from "$app";
  import Modal from "../Modal.svelte";

  interface Props {
    rootFolder: string;
    onclose: () => void;
    /** Rejects the chosen paths; resolves true on success. */
    onapply: (paths: string[]) => Promise<boolean>;
  }

  let { rootFolder, onclose, onapply }: Props = $props();

  const SIGMA_THRESHOLD = 2.0;

  let loading = $state(true);
  let error = $state("");
  let applying = $state(false);
  let results = $state<app.SuggestResult[]>([]);
  const selected = new SvelteSet<string>();

  onMount(async () => {
    try {
      results = (await SuggestRejects(rootFolder, SIGMA_THRESHOLD)) ?? [];
      for (const r of results) selected.add(r.frame.nasPath);
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
    }
  });

  function toggle(path: string) {
    if (selected.has(path)) selected.delete(path);
    else selected.add(path);
  }

  async function apply() {
    if (selected.size === 0 || applying) return;
    applying = true;
    const ok = await onapply([...selected]);
    applying = false;
    if (ok) onclose();
  }
</script>

<Modal title="Suggested rejects" {onclose} busy={applying} width="min(680px, 92vw)">
  {#if loading}
    <p class="status" role="status">Analyzing quality metrics…</p>
  {:else if error}
    <p class="status error" role="alert">Could not compute suggestions: {error}</p>
  {:else if results.length === 0}
    <p class="status">
      No outliers found. Either all frames are good quality, or not enough frames have been analyzed
      (run ✦ Analyze on a group first).
    </p>
  {:else}
    <p class="desc">
      {results.length} frame{results.length !== 1 ? "s" : ""} with FWHM more than {SIGMA_THRESHOLD}σ
      above their group median. Untick any you want to keep.
    </p>
    <div class="list">
      {#each results as r (r.frame.nasPath)}
        <label class="row" class:deselected={!selected.has(r.frame.nasPath)}>
          <input
            type="checkbox"
            checked={selected.has(r.frame.nasPath)}
            onchange={() => toggle(r.frame.nasPath)}
          />
          <span class="name">{r.frame.fileName}</span>
          <span class="obj">{r.frame.object}</span>
          <span
            class="fwhm"
            title="FWHM {r.frame.fwhm.toFixed(2)} vs median {r.groupMedian.toFixed(
              2,
            )} (σ={r.groupSigma.toFixed(2)})"
          >
            {r.frame.fwhm.toFixed(2)}
            {r.frame.fwhmUnit} · {r.sigmas.toFixed(1)}σ
          </span>
        </label>
      {/each}
    </div>
  {/if}

  {#snippet actions()}
    {#if !loading && results.length > 0}
      <span class="sel-count">{selected.size} selected</span>
    {/if}
    <button class="btn-ghost" onclick={onclose} disabled={applying}>
      {results.length > 0 ? "Cancel" : "Close"}
    </button>
    {#if !loading && results.length > 0}
      <button class="btn-primary" disabled={selected.size === 0 || applying} onclick={apply}>
        {applying ? "Rejecting…" : `Reject ${selected.size} frame${selected.size !== 1 ? "s" : ""}`}
      </button>
    {/if}
  {/snippet}
</Modal>

<style>
  .status {
    margin: 0;
    color: var(--text-secondary);
    font-size: var(--fs-md);
  }
  .status.error {
    color: var(--danger);
  }
  .desc {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 50vh;
    overflow-y: auto;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 5px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: var(--fs-sm);
    transition: background 0.1s;
  }
  .row:hover {
    background: var(--bg-row-hover);
  }
  .row input {
    accent-color: var(--accent);
    flex-shrink: 0;
  }
  .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: "Consolas", monospace;
    color: var(--text-primary);
  }
  .obj {
    width: 100px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-secondary);
  }
  .fwhm {
    width: 130px;
    text-align: right;
    color: var(--danger);
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }
  .row.deselected .name,
  .row.deselected .obj,
  .row.deselected .fwhm {
    color: var(--text-dim);
    text-decoration: line-through;
  }
  .sel-count {
    margin-right: auto;
    align-self: center;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
</style>
