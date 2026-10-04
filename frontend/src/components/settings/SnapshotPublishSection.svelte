<script lang="ts">
  // Desktop: publish a library snapshot for the read-only server viewer.
  import { onMount } from "svelte";
  import { GetSnapshotStatus, PublishSnapshot } from "$app";
  import type * as app from "$models/app";
  import { savePref } from "../../lib/prefs";
  import { formatDate } from "../../lib/utils";

  const PREF_SNAPSHOT_ENABLED = "snapshot_enabled";

  let status = $state<app.SnapshotStatus | null>(null);
  let publishing = $state(false);

  async function refresh() {
    try {
      status = await GetSnapshotStatus();
    } catch {
      /* keep the last known status */
    }
  }

  async function publishNow() {
    publishing = true;
    try {
      await PublishSnapshot();
    } catch {
      /* the error is in the refreshed status */
    } finally {
      publishing = false;
      await refresh();
    }
  }

  function toggle(e: Event) {
    const on = (e.currentTarget as HTMLInputElement).checked;
    savePref(PREF_SNAPSHOT_ENABLED, String(on));
    if (status) status.enabled = on;
    if (on) void publishNow();
  }

  onMount(() => {
    void refresh();
  });
</script>

<section class="card">
  <h2 class="section-title">Server viewer</h2>
  <p class="section-desc">
    The headless server build is a read-only viewer of your final images and the Sky Atlas. It reads
    a copy of this library that the app writes into the library folder, so the server never needs
    this database. The copy is updated a couple of minutes after changes, and when the app closes.
  </p>

  <label class="toggle-row">
    <input
      type="checkbox"
      checked={status?.enabled ?? false}
      onchange={toggle}
      disabled={!status}
    />
    Publish a library snapshot for the server viewer
  </label>

  {#if status?.enabled}
    <div class="info-row">
      <span class="info-label">File</span>
      <span class="info-value mono" title={status.path}>{status.path || "—"}</span>
    </div>
    <div class="info-row">
      <span class="info-label">Updated</span>
      <span class="info-value">
        {status.lastPublished ? formatDate(status.lastPublished) : "Not yet this session"}
      </span>
      <button class="btn-ghost small" onclick={publishNow} disabled={publishing}>
        {publishing ? "Publishing…" : "Publish now"}
      </button>
    </div>
    {#if status.lastError}
      <p class="snapshot-error" role="alert">Last publish failed: {status.lastError}</p>
    {/if}
  {/if}
</section>

<style>
  .toggle-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    cursor: pointer;
  }

  .snapshot-error {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--danger);
  }
</style>
