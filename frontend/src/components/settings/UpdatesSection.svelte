<script lang="ts">
  import { onMount } from "svelte";
  import { Events } from "@wailsio/runtime";
  import { CheckForUpdate, GetAppVersion, InstallUpdate, OpenReleasePage, RestartApp } from "$app";
  import type { UpdateInfo, UpdateProgress } from "../../lib/types";
  import { toast } from "../../lib/toast.svelte";

  interface Props {
    /** A running index blocks the restart so it isn't cut short. */
    indexRunning: boolean;
  }

  let { indexRunning }: Props = $props();

  let version = $state("");
  let info = $state<UpdateInfo | null>(null);
  let checking = $state(false);
  let installing = $state(false);
  let installed = $state(false);
  let progress = $state<UpdateProgress | null>(null);
  let error = $state("");

  onMount(() => {
    void GetAppVersion().then((v) => (version = v));
    const off = Events.On("update:progress", (event) => {
      progress = event.data as UpdateProgress;
    });
    return off;
  });

  async function check() {
    checking = true;
    error = "";
    try {
      info = await CheckForUpdate();
    } catch (e) {
      info = null;
      error = String(e);
    } finally {
      checking = false;
    }
  }

  async function install() {
    installing = true;
    error = "";
    progress = null;
    try {
      await InstallUpdate();
      installed = true;
    } catch (e) {
      error = String(e);
    } finally {
      installing = false;
    }
  }

  async function restart() {
    try {
      await RestartApp();
    } catch (e) {
      toast.error(`Couldn't restart: ${String(e)}`);
    }
  }

  async function openPage() {
    try {
      await OpenReleasePage();
    } catch (e) {
      toast.error(`Couldn't open the release page: ${String(e)}`);
    }
  }

  function mb(bytes: number): string {
    return (bytes / (1024 * 1024)).toFixed(1);
  }

  let percent = $derived(
    progress && progress.phase === "downloading" && progress.total > 0
      ? Math.min(100, (progress.done / progress.total) * 100)
      : null,
  );

  let statusText = $derived.by(() => {
    if (!progress) return "Starting…";
    if (progress.phase === "installing") {
      return info?.kind === "deb" ? "Waiting for the package installer…" : "Installing…";
    }
    if (progress.phase === "downloading") {
      return progress.total > 0
        ? `Downloading… ${mb(progress.done)} / ${mb(progress.total)} MB`
        : `Downloading… ${mb(progress.done)} MB`;
    }
    return "Starting…";
  });
</script>

<section class="card">
  <h2 class="section-title">Updates</h2>
  <p class="section-desc">
    Eirin only looks for a new version when you ask here. It never checks in the background or
    prompts at startup.
  </p>

  <div class="info-row">
    <span class="info-label">Version</span>
    <span class="info-value">{version || "…"}</span>
    <button class="btn-ghost small" onclick={check} disabled={checking || installing}>
      {checking ? "Checking…" : "Check for updates"}
    </button>
  </div>

  {#if error}
    <div class="update-box error" role="alert">{error}</div>
  {/if}

  {#if installed}
    <div class="update-box ok" role="status">
      <span class="update-msg">
        Version {info?.latest} is installed. Restart Eirin to start using it.
        {#if indexRunning}An index is running; restart once it finishes.{/if}
      </span>
      <button class="btn-primary small" onclick={restart} disabled={indexRunning}>
        Restart now
      </button>
    </div>
  {:else if installing}
    <div class="update-box" role="status">
      <span class="update-msg">{statusText}</span>
      <div class="bar" class:indeterminate={percent === null}>
        <div class="bar-fill" style:width={percent === null ? undefined : `${percent}%`}></div>
      </div>
    </div>
  {:else if info}
    {#if info.available}
      <div class="update-box" role="status">
        <span class="update-msg">
          <strong>Version {info.latest}</strong> is available (you have {info.current}).
        </span>
        {#if info.canInstall}
          <button class="btn-primary small" onclick={install}>
            {info.kind === "deb" ? "Download and install…" : "Download and install"}
          </button>
        {:else}
          <button class="btn-ghost small" onclick={openPage}>Open release page</button>
        {/if}
      </div>
      {#if !info.canInstall && info.reason}
        <p class="action-hint">{info.reason} Download the new version from the release page.</p>
      {:else if info.kind === "deb"}
        <p class="action-hint">
          Installed from a .deb package: your system will ask for your password to install the
          update.
        </p>
      {/if}
      {#if info.notes}
        <details class="notes">
          <summary>What's new</summary>
          <pre>{info.notes}</pre>
        </details>
      {/if}
    {:else}
      <p class="action-hint" role="status">
        {#if info.reason}
          {info.reason} The latest release is {info.latest}.
        {:else}
          You're up to date.
        {/if}
      </p>
    {/if}
  {/if}
</section>

<style>
  .update-box {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px 12px;
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: 5px;
    background: var(--bg-base);
  }
  .update-box.ok {
    border-color: var(--success);
    background: color-mix(in srgb, var(--success) 10%, transparent);
  }
  .update-box.error {
    border-color: var(--danger);
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    color: var(--danger);
    font-size: var(--fs-xs);
    word-break: break-word;
    user-select: text;
  }
  .update-msg {
    flex: 1;
    min-width: 0;
    font-size: var(--fs-sm);
    color: var(--text-primary);
  }

  .bar {
    flex-basis: 100%;
    height: 4px;
    border-radius: 2px;
    background: var(--border);
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    width: 0;
    background: var(--accent);
    transition: width 0.1s linear;
  }
  .bar.indeterminate .bar-fill {
    width: 35%;
    animation: slide 1.2s ease-in-out infinite;
  }
  @keyframes slide {
    from {
      transform: translateX(-100%);
    }
    to {
      transform: translateX(300%);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .bar.indeterminate .bar-fill {
      animation: none;
    }
  }

  .notes summary {
    cursor: pointer;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .notes pre {
    margin: 6px 0 0;
    padding: 8px 10px;
    max-height: 220px;
    overflow: auto;
    font-family: inherit;
    font-size: var(--fs-xs);
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-word;
    color: var(--text-secondary);
    background: var(--bg-base);
    border-radius: 5px;
    user-select: text;
  }
</style>
