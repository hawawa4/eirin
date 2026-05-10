<script lang="ts">
  interface Props {
    rootFolder: string;
    indexRunning: boolean;
    onselectfolder: () => void;
    onbuildindex: () => void;
  }

  let { rootFolder, indexRunning, onselectfolder, onbuildindex }: Props = $props();
</script>

<header>
  <span class="logo">✦ Eirin</span>
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
  }

  .logo {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--accent);
    letter-spacing: 0.05em;
  }

  .header-right {
    -webkit-app-region: no-drag;
    display: flex;
    align-items: center;
  }
</style>
