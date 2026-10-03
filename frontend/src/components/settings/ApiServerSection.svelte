<script lang="ts">
  import type { AppInfo } from "../../lib/types";
  import { copyText } from "../../lib/clipboard";

  interface Props {
    appInfo: AppInfo;
  }

  let { appInfo }: Props = $props();

  // Routes registered in internal/app/server.go.
  const ENDPOINTS: { path: string; desc: string }[] = [
    { path: "/api/status", desc: "server status and configured root folder (JSON)" },
    { path: "/api/frames", desc: "all indexed frames under the root folder (JSON)" },
    { path: "/api/image?path=<file>", desc: "a raster file (PNG/TIFF) under the root folder" },
  ];
</script>

<section class="card">
  <h2 class="section-title">API Server</h2>
  <p class="section-desc">
    A local HTTP server that exposes the library over a read-only REST API. Useful for scripting and
    external tool integration.
  </p>

  <div class="info-row">
    <span class="info-label">URL</span>
    <span class="info-value mono">{appInfo.serverUrl}</span>
    <button
      class="btn-ghost small"
      onclick={() => copyText(appInfo.serverUrl, "URL copied")}
      disabled={!appInfo.serverUrl}>Copy URL</button
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
    {#each ENDPOINTS as ep (ep.path)}
      <div class="endpoint">
        <code>GET {appInfo.serverUrl}{ep.path}</code>
        <span class="endpoint-desc">{ep.desc}</span>
      </div>
    {/each}
  </div>

  <p class="action-hint">
    Set the <code>EIRIN_PORT</code> environment variable before launching to use a custom port.
  </p>
</section>

<style>
  .endpoints {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-top: 4px;
  }

  .endpoints-label {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    margin: 0 0 2px;
  }

  .endpoint {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .endpoint code {
    font-size: var(--fs-xs);
    color: var(--accent);
    font-family: monospace;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 3px 8px;
    user-select: text;
    word-break: break-all;
  }

  .endpoint-desc {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    padding-left: 8px;
  }
</style>
