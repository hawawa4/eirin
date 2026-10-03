<script lang="ts">
  import { BackupDatabase } from "$app";
  import { copyPath, revealPath } from "../../lib/shell/fileActions";

  interface Props {
    dbPath: string;
    desktopMode: boolean;
  }

  let { dbPath, desktopMode }: Props = $props();

  let backingUp = $state(false);
  /** Last backup outcome; stays visible until dismissed. */
  let backupResult = $state<{ path: string; error: string } | null>(null);

  async function doBackup() {
    backingUp = true;
    backupResult = null;
    try {
      const path = await BackupDatabase();
      backupResult = { path, error: "" };
    } catch (e) {
      backupResult = { path: "", error: String(e) };
    } finally {
      backingUp = false;
    }
  }
</script>

<section class="card">
  <h2 class="section-title">Database</h2>
  <p class="section-desc">SQLite database storing all indexed frame metadata and preferences.</p>

  <div class="info-row">
    <span class="info-label">Path</span>
    <span class="info-value mono" title={dbPath}>{dbPath}</span>
    {#if desktopMode && dbPath}
      <button class="btn-ghost small" onclick={() => revealPath(dbPath)}>Show in folder</button>
    {/if}
    <button class="btn-ghost small" onclick={() => copyPath(dbPath)} disabled={!dbPath}>
      Copy path
    </button>
  </div>
  <div class="info-row">
    <span class="info-label">Backup</span>
    <button class="btn-ghost small" onclick={doBackup} disabled={backingUp}>
      {backingUp ? "Backing up…" : "Back up now"}
    </button>
  </div>

  {#if backupResult}
    <div class="backup-result" class:error={!!backupResult.error} role="status">
      <span class="backup-msg">
        {#if backupResult.error}
          Backup failed: {backupResult.error}
        {:else}
          Backup saved to <span class="mono">{backupResult.path}</span>
        {/if}
      </span>
      {#if !backupResult.error && desktopMode}
        <button class="btn-ghost small" onclick={() => revealPath(backupResult!.path)}>
          Show in folder
        </button>
      {/if}
      <button
        class="btn-ghost small"
        onclick={() => (backupResult = null)}
        aria-label="Dismiss backup message">Dismiss</button
      >
    </div>
  {/if}

  <p class="action-hint">A timestamped copy is also made automatically every 30 minutes.</p>
</section>

<style>
  .backup-result {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border: 1px solid var(--success);
    border-radius: 5px;
    background: color-mix(in srgb, var(--success) 10%, transparent);
  }
  .backup-result.error {
    border-color: var(--danger);
    background: color-mix(in srgb, var(--danger) 10%, transparent);
  }

  .backup-msg {
    flex: 1;
    min-width: 0;
    font-size: var(--fs-xs);
    color: var(--success);
    word-break: break-all;
    user-select: text;
  }
  .backup-result.error .backup-msg {
    color: var(--danger);
  }
</style>
