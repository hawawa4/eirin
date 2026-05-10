<script lang="ts">
  import type { AppMode } from "../lib/types";
  import ModeSelector from "./ModeSelector.svelte";

  interface Props {
    rootFolder: string;
    indexRunning: boolean;
    appMode: AppMode;
    onselectfolder: () => void;
    onbuildindex: () => void;
    onmodechange: (mode: AppMode) => void;
  }

  let { rootFolder, indexRunning, appMode, onselectfolder, onbuildindex, onmodechange }: Props =
    $props();
</script>

<header>
  <span class="logo">✦ Eirin</span>

  {#if rootFolder}
    <div class="mode-area">
      <ModeSelector mode={appMode} {onmodechange} />
    </div>
  {/if}

  <div class="header-right">
    {#if rootFolder}
      <button
        class="btn-secondary"
        onclick={onbuildindex}
        disabled={indexRunning}
        title="Scan all subfolders and index FITS headers. Re-run to pick up new files."
      >
        {indexRunning ? "Indexing…" : "Build Index"}
      </button>
    {/if}
    <button class="btn-primary" onclick={onselectfolder}>
      {rootFolder ? "Change Root Folder" : "Select Root Folder"}
    </button>
  </div>
</header>

<style>
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 20px;
    height: 52px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    -webkit-app-region: drag;
    gap: 16px;
  }

  .logo {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--accent);
    letter-spacing: 0.05em;
    flex-shrink: 0;
  }

  .mode-area {
    flex: 1;
    display: flex;
    justify-content: center;
    -webkit-app-region: no-drag;
  }

  .header-right {
    -webkit-app-region: no-drag;
    display: flex;
    align-items: center;
    flex-shrink: 0;
  }
</style>
