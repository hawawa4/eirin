<script lang="ts">
  // Server viewer: which library snapshot is being shown.
  import { onMount } from "svelte";
  import { GetSnapshotStatus } from "$app";
  import type * as app from "$models/app";
  import { formatDate } from "../../lib/utils";

  const REFRESH_MS = 30_000;

  let status = $state<app.SnapshotStatus | null>(null);

  async function refresh() {
    try {
      status = await GetSnapshotStatus();
    } catch {
      /* keep the last known status */
    }
  }

  onMount(() => {
    void refresh();
    const timer = setInterval(refresh, REFRESH_MS);
    return () => clearInterval(timer);
  });
</script>

<section class="card">
  <h2 class="section-title">Library snapshot</h2>
  <p class="section-desc">
    This is a read-only viewer. It shows the stacked and processed images from the library snapshot
    the desktop app publishes into the library folder, and picks up new snapshots automatically.
  </p>

  {#if status}
    <div class="info-row">
      <span class="info-label">Library</span>
      <span class="info-value mono" title={status.root}>{status.root}</span>
    </div>
    {#if status.loaded}
      <div class="info-row">
        <span class="info-label">From</span>
        <span class="info-value mono" title={status.sourceRoot}>{status.sourceRoot}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Images</span>
        <span class="info-value">{status.frames}</span>
      </div>
      <div class="info-row">
        <span class="info-label">Published</span>
        <span class="info-value">{status.publishedAt ? formatDate(status.publishedAt) : "—"}</span>
      </div>
    {:else}
      <p class="action-hint">
        No snapshot yet. In the desktop app, turn on <strong>Settings → Server viewer</strong>.
      </p>
    {/if}
    {#if status.lastError}
      <p class="snapshot-error" role="alert">
        Couldn't load the latest snapshot: {status.lastError}
      </p>
    {/if}
  {/if}
</section>

<style>
  .snapshot-error {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--danger);
  }
</style>
