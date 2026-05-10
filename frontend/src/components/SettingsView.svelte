<script lang="ts">
  import type { AppInfo, IndexProgress } from "../lib/types";

  interface Props {
    rootFolder: string;
    appInfo: AppInfo;
    indexRunning: boolean;
    indexProgress: IndexProgress | null;
    onselectfolder: () => void;
    onbuildindex: () => void;
  }

  let { rootFolder, appInfo, indexRunning, indexProgress, onselectfolder, onbuildindex }: Props =
    $props();

  let copied = $state("");

  async function copyToClipboard(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      copied = key;
      setTimeout(() => {
        copied = "";
      }, 1500);
    } catch {
      /* clipboard not available */
    }
  }
</script>

<div class="settings-view">
  <div class="settings-body">
    <!-- ── Root Folder ───────────────────────────────────────────────────── -->
    <section class="card">
      <h2 class="section-title">Root Folder</h2>
      <p class="section-desc">
        The NAS folder that Eirin treats as the root of your astrophotography library.
      </p>

      <div class="path-row">
        <span class="path-value" title={rootFolder || "Not set"}>
          {rootFolder || "No folder selected"}
        </span>
        <button class="btn-secondary" onclick={onselectfolder}>
          {rootFolder ? "Change" : "Select"}
        </button>
      </div>

      {#if rootFolder}
        <div class="action-row">
          <button class="btn-primary" onclick={onbuildindex} disabled={indexRunning}>
            {indexRunning ? "Indexing…" : "Build Index"}
          </button>
          <p class="action-hint">
            Scans all subfolders and reads FITS headers for any files not yet in the database.
            Re-run to pick up new files.
          </p>
        </div>
      {/if}
    </section>

    <!-- ── Database ──────────────────────────────────────────────────────── -->
    <section class="card">
      <h2 class="section-title">Database</h2>
      <p class="section-desc">
        SQLite database storing all indexed frame metadata and preferences.
      </p>

      <div class="info-row">
        <span class="info-label">Path</span>
        <span class="info-value mono" title={appInfo.dbPath}>{appInfo.dbPath}</span>
        <button
          class="btn-ghost"
          onclick={() => copyToClipboard(appInfo.dbPath, "db")}
          title="Copy path"
        >
          {copied === "db" ? "✓" : "⎘"}
        </button>
      </div>
    </section>

    <!-- ── API Server ────────────────────────────────────────────────────── -->
    <section class="card">
      <h2 class="section-title">API Server</h2>
      <p class="section-desc">
        A local HTTP server that exposes the library over a REST API. Useful for scripting and
        external tool integration.
      </p>

      <div class="info-row">
        <span class="info-label">URL</span>
        <span class="info-value mono">{appInfo.serverUrl}</span>
        <button
          class="btn-ghost"
          onclick={() => copyToClipboard(appInfo.serverUrl, "url")}
          title="Copy URL"
        >
          {copied === "url" ? "✓" : "⎘"}
        </button>
      </div>

      <div class="info-row">
        <span class="info-label">Port</span>
        <span class="info-value">{appInfo.serverPort}</span>
      </div>

      <div class="info-row">
        <span class="info-label">Source</span>
        <span class="info-value">{appInfo.portSource}</span>
      </div>

      <div class="endpoints">
        <p class="endpoints-label">Endpoints</p>
        <code>GET {appInfo.serverUrl}/api/status</code>
        <code>GET {appInfo.serverUrl}/api/frames</code>
      </div>

      <p class="env-hint">
        Set the <code>EIRIN_PORT</code> environment variable before launching to use a custom port.
      </p>
    </section>
  </div>
</div>

<style>
  .settings-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-base);
  }

  .settings-body {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    max-width: 720px;
    width: 100%;
    margin: 0 auto;
  }

  /* ── Card ────────────────────────────────────────────────────────────── */

  .card {
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .section-title {
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .section-desc {
    font-size: 0.82rem;
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.5;
  }

  /* ── Path / action rows ──────────────────────────────────────────────── */

  .path-row {
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 8px 12px;
  }

  .path-value {
    flex: 1;
    font-family: monospace;
    font-size: 0.82rem;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .action-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .action-hint {
    font-size: 0.78rem;
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.4;
  }

  /* ── Info rows ───────────────────────────────────────────────────────── */

  .info-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 0;
    border-bottom: 1px solid var(--border);
  }

  .info-row:last-of-type {
    border-bottom: none;
  }

  .info-label {
    font-size: 0.78rem;
    color: var(--text-secondary);
    width: 56px;
    flex-shrink: 0;
  }

  .info-value {
    flex: 1;
    font-size: 0.82rem;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }

  .mono {
    font-family: monospace;
  }

  /* ── Endpoints ───────────────────────────────────────────────────────── */

  .endpoints {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-top: 4px;
  }

  .endpoints-label {
    font-size: 0.78rem;
    color: var(--text-secondary);
    margin: 0 0 2px;
  }

  .endpoints code {
    font-size: 0.78rem;
    color: var(--accent-dim);
    font-family: monospace;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 3px 8px;
  }

  .env-hint {
    font-size: 0.78rem;
    color: var(--text-secondary);
    margin: 4px 0 0;
    line-height: 1.4;
  }

  .env-hint code {
    font-family: monospace;
    color: var(--accent-dim);
    background: var(--bg-base);
    border-radius: 3px;
    padding: 1px 4px;
    font-size: 0.78rem;
  }

  /* ── Buttons ─────────────────────────────────────────────────────────── */

  .btn-ghost {
    background: transparent;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-secondary);
    font-size: 0.82rem;
    padding: 3px 8px;
    cursor: pointer;
    flex-shrink: 0;
    transition: color 0.15s, border-color 0.15s;
  }

  .btn-ghost:hover {
    color: var(--text-primary);
    border-color: var(--accent-dim);
  }
</style>
