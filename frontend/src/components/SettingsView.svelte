<script lang="ts">
  import { onMount } from "svelte";
  import {
    CheckSiril,
    SelectSirilExecutable,
    SetSirilPath,
    SelectProjectsFolder,
    SetProjectsFolder,
  } from "../../wailsjs/go/app/App.js";
  import type { AppInfo, IndexProgress, SirilInfo } from "../lib/types";

  interface Props {
    rootFolder: string;
    projectsFolder: string;
    appInfo: AppInfo;
    indexRunning: boolean;
    indexProgress: IndexProgress | null;
    onselectfolder: () => void;
    onbuildindex: () => void;
    onsirilchange: (info: SirilInfo) => void;
    onprojectsfolderset: (path: string) => void;
  }

  let {
    rootFolder,
    projectsFolder,
    appInfo,
    indexRunning,
    indexProgress,
    onselectfolder,
    onbuildindex,
    onsirilchange,
    onprojectsfolderset,
  }: Props = $props();

  // ── Projects folder ────────────────────────────────────────────────────────
  async function browseProjectsFolder() {
    const path = await SelectProjectsFolder();
    if (!path) return;
    await SetProjectsFolder(path);
    onprojectsfolderset(path);
  }

  // ── Clipboard copy ────────────────────────────────────────────────────────
  let copied = $state("");

  async function copyToClipboard(text: string, key: string) {
    try {
      await navigator.clipboard.writeText(text);
      copied = key;
      setTimeout(() => (copied = ""), 1500);
    } catch {
      /* clipboard not available */
    }
  }

  // ── Siril ─────────────────────────────────────────────────────────────────
  let sirilInfo = $state<SirilInfo>({ executable: "siril", version: "…", available: false });
  let sirilChecking = $state(false);
  let sirilPathInput = $state("");
  let sirilPathDirty = $state(false);

  onMount(async () => {
    await refreshSiril();
  });

  async function refreshSiril() {
    sirilChecking = true;
    try {
      sirilInfo = await CheckSiril();
      sirilPathInput = sirilInfo.executable;
      sirilPathDirty = false;
      onsirilchange(sirilInfo);
    } finally {
      sirilChecking = false;
    }
  }

  async function browseSiril() {
    const path = await SelectSirilExecutable();
    if (!path) return;
    await SetSirilPath(path);
    await refreshSiril();
  }

  async function saveSirilPath() {
    await SetSirilPath(sirilPathInput);
    sirilPathDirty = false;
    await refreshSiril();
  }

  async function resetSirilPath() {
    sirilPathInput = "siril";
    await SetSirilPath("");
    sirilPathDirty = false;
    await refreshSiril();
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

    <!-- ── Projects Folder ─────────────────────────────────────────────────── -->
    <section class="card">
      <h2 class="section-title">Projects Folder</h2>
      <p class="section-desc">
        Local folder where Siril projects are stored. Each project gets its own subfolder
        containing a <code>lights/</code> directory with symlinks or copies of your frames.
      </p>

      <div class="path-row">
        <span class="path-value" title={projectsFolder || "Not set"}>
          {projectsFolder || "No folder selected"}
        </span>
        <button class="btn-secondary" onclick={browseProjectsFolder}>
          {projectsFolder ? "Change" : "Select"}
        </button>
      </div>
    </section>

    <!-- ── Siril ─────────────────────────────────────────────────────────── -->
    <section class="card">
      <h2 class="section-title">Siril</h2>
      <p class="section-desc">
        Siril is used for astrophotography processing. Eirin can open files directly in Siril
        from the right-click context menu.
      </p>

      <div class="siril-status">
        <span class="status-dot" class:dot-ok={sirilInfo.available} class:dot-err={!sirilInfo.available}
        ></span>
        <span class="status-version">
          {#if sirilChecking}
            Checking…
          {:else}
            {sirilInfo.version}
          {/if}
        </span>
        <button class="btn-ghost" onclick={refreshSiril} disabled={sirilChecking} title="Re-check">
          ↺
        </button>
      </div>

      <div class="path-row">
        <input
          class="path-input"
          type="text"
          bind:value={sirilPathInput}
          oninput={() => (sirilPathDirty = true)}
          placeholder="siril"
          spellcheck="false"
        />
        <button class="btn-secondary" onclick={browseSiril}>Browse…</button>
      </div>

      {#if sirilPathDirty}
        <div class="action-row">
          <button class="btn-primary" onclick={saveSirilPath}>Save</button>
          <button
            class="btn-ghost"
            onclick={() => {
              sirilPathInput = sirilInfo.executable;
              sirilPathDirty = false;
            }}
          >
            Cancel
          </button>
        </div>
      {:else if sirilInfo.executable !== "siril" && sirilInfo.executable !== ""}
        <button class="btn-ghost reset-btn" onclick={resetSirilPath}>Reset to default</button>
      {/if}

      <p class="action-hint">
        Leave blank or set to <code>siril</code> to use the system PATH. Use Browse to locate a
        custom binary.
      </p>
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
          title="Copy path">{copied === "db" ? "✓" : "⎘"}</button
        >
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
          title="Copy URL">{copied === "url" ? "✓" : "⎘"}</button
        >
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

  /* ── Path rows ───────────────────────────────────────────────────────── */

  .path-row {
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 6px 12px;
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

  .path-input {
    flex: 1;
    background: transparent;
    border: none;
    outline: none;
    font-family: monospace;
    font-size: 0.82rem;
    color: var(--text-primary);
    min-width: 0;
  }

  .path-input::placeholder {
    color: var(--text-secondary);
  }

  /* ── Action rows ─────────────────────────────────────────────────────── */

  .action-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .action-hint {
    font-size: 0.78rem;
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.4;
  }

  .action-hint code {
    font-family: monospace;
    background: var(--bg-base);
    border-radius: 3px;
    padding: 1px 4px;
    font-size: 0.78rem;
    color: var(--accent);
  }

  /* ── Siril status ────────────────────────────────────────────────────── */

  .siril-status {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-ok {
    background: #5fba7d;
    box-shadow: 0 0 4px #5fba7d88;
  }

  .dot-err {
    background: var(--danger, #e06c75);
  }

  .status-version {
    flex: 1;
    font-size: 0.82rem;
    color: var(--text-primary);
    font-family: monospace;
  }

  .reset-btn {
    align-self: flex-start;
    font-size: 0.75rem;
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
    color: var(--accent);
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
    color: var(--accent);
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
    border-color: var(--accent);
  }

  .btn-ghost:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
</style>
