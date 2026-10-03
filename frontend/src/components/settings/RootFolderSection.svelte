<script lang="ts">
  import Modal from "../Modal.svelte";

  interface Props {
    rootFolder: string;
    desktopMode: boolean;
    indexRunning: boolean;
    onselectfolder: () => void;
    /** Non-forced scan: picks up new/changed files only. */
    onscan: () => void;
    /** Forced scan: re-reads every file. */
    onrescanall: () => void;
    onreveal?: (path: string) => void;
  }

  let {
    rootFolder,
    desktopMode,
    indexRunning,
    onselectfolder,
    onscan,
    onrescanall,
    onreveal,
  }: Props = $props();

  let confirmRescan = $state(false);
</script>

<section class="card">
  <h2 class="section-title">Root Folder</h2>
  <p class="section-desc">
    The NAS folder that Eirin treats as the root of your astrophotography library.
  </p>

  <div class="path-row">
    <span class="path-value" title={rootFolder || "Not set"}>
      {rootFolder || "No folder selected"}
    </span>
    {#if rootFolder && onreveal}
      <button class="btn-ghost small" onclick={() => onreveal(rootFolder)}>Show in folder</button>
    {/if}
    {#if desktopMode}
      <button class="btn-secondary" onclick={onselectfolder}>
        {rootFolder ? "Change…" : "Select…"}
      </button>
    {/if}
  </div>
  {#if !desktopMode}
    <p class="action-hint">
      In server mode the root folder comes from the server's preferences database and can't be
      changed here.
    </p>
  {/if}

  {#if rootFolder}
    <div class="action-row">
      <div class="btn-group">
        <button class="btn-primary" onclick={() => onscan()} disabled={indexRunning}>
          {indexRunning ? "Scanning…" : "Scan for new files"}
        </button>
        <button
          class="btn-secondary"
          onclick={() => (confirmRescan = true)}
          disabled={indexRunning}
          title="Re-read every file's FITS header, including ones already in the library"
        >
          Re-read all files…
        </button>
      </div>
    </div>
    <p class="action-hint">
      <strong>Scan for new files</strong> walks every subfolder and reads FITS headers only for
      files that aren't in the library yet. It runs automatically after you change the root folder
      or import new frames. <strong>Re-read all files</strong> reads every header again from scratch —
      only needed if the library looks wrong.
    </p>
  {/if}
</section>

{#if confirmRescan}
  <Modal
    title="Re-read all files?"
    onclose={() => {
      confirmRescan = false;
    }}
  >
    <p class="modal-text">
      This re-reads the FITS header of <strong>every</strong> file under the root folder, including files
      already in the library. On a NAS with many frames this can take a long time.
    </p>
    <p class="modal-text">
      Plate-solve (WCS) results are kept. You can keep using Eirin and cancel the scan at any time.
    </p>
    {#snippet actions()}
      <button
        class="btn-ghost"
        data-autofocus
        onclick={() => {
          confirmRescan = false;
        }}>Cancel</button
      >
      <button
        class="btn-primary"
        onclick={() => {
          confirmRescan = false;
          onrescanall();
        }}>Re-read all files</button
      >
    {/snippet}
  </Modal>
{/if}

<style>
  .modal-text {
    margin: 0 0 10px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    line-height: 1.5;
  }
  .modal-text strong {
    color: var(--text-primary);
  }
</style>
