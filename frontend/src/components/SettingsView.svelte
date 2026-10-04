<script lang="ts">
  import type { AppInfo, SirilInfo } from "../lib/types";
  import { openFolder } from "../lib/shell/fileActions";
  import RootFolderSection from "./settings/RootFolderSection.svelte";
  import ProjectsFolderSection from "./settings/ProjectsFolderSection.svelte";
  import SirilSection from "./settings/SirilSection.svelte";
  import AppearanceSection from "./settings/AppearanceSection.svelte";
  import DatabaseSection from "./settings/DatabaseSection.svelte";
  import ApiServerSection from "./settings/ApiServerSection.svelte";
  import SnapshotPublishSection from "./settings/SnapshotPublishSection.svelte";
  import SnapshotStatusSection from "./settings/SnapshotStatusSection.svelte";

  interface Props {
    rootFolder: string;
    projectsFolder: string;
    appInfo: AppInfo;
    indexRunning: boolean;
    onselectfolder: () => void;
    /** Scan for new files (non-forced). */
    onbuildindex: () => void;
    /** Re-read every file (forced). */
    onrebuildindex: () => void;
    onsirilchange: (info: SirilInfo) => void;
    onprojectsfolderset: (path: string) => void;
  }

  let {
    rootFolder,
    projectsFolder,
    appInfo,
    indexRunning,
    onselectfolder,
    onbuildindex,
    onrebuildindex,
    onsirilchange,
    onprojectsfolderset,
  }: Props = $props();

  let desktopMode = $derived(appInfo.capabilities.desktopMode);
  let readOnly = $derived(appInfo.capabilities.readOnly);
</script>

<div class="settings-view">
  <div class="settings-body">
    {#if readOnly}
      <SnapshotStatusSection />
    {:else}
      <RootFolderSection
        {rootFolder}
        {desktopMode}
        {indexRunning}
        {onselectfolder}
        onscan={() => onbuildindex()}
        onrescanall={() => onrebuildindex()}
        onreveal={desktopMode ? (p) => openFolder(p) : undefined}
      />
    {/if}

    {#if desktopMode}
      <ProjectsFolderSection {projectsFolder} {onprojectsfolderset} />
      <SirilSection {onsirilchange} />
      <SnapshotPublishSection />
    {/if}

    <AppearanceSection />
    {#if !readOnly}
      <DatabaseSection dbPath={appInfo.dbPath} {desktopMode} />
    {/if}
    <ApiServerSection {appInfo} />
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
    max-width: 760px;
    width: 100%;
    margin: 0 auto;
  }

  /* ── Shared section styles (used by the components in ./settings/) ───── */

  .settings-view :global(.card) {
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .settings-view :global(.section-title) {
    font-size: var(--fs-lg);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }

  .settings-view :global(.section-desc) {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.5;
  }

  .settings-view :global(.path-row) {
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--bg-base);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 6px 12px;
  }
  .settings-view :global(.path-row .btn-secondary) {
    margin-right: 0;
  }

  .settings-view :global(.path-value),
  .settings-view :global(.path-input) {
    flex: 1;
    min-width: 0;
    font-family: monospace;
    font-size: var(--fs-sm);
    color: var(--text-primary);
  }
  .settings-view :global(.path-value) {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    user-select: text;
  }
  .settings-view :global(.path-input) {
    background: transparent;
    border: none;
    outline: none;
  }
  .settings-view :global(.path-input::placeholder) {
    color: var(--text-secondary);
  }
  .settings-view :global(.path-row:focus-within) {
    border-color: var(--accent);
  }

  .settings-view :global(.action-row) {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .settings-view :global(.btn-group) {
    display: flex;
    gap: 6px;
    flex-shrink: 0;
  }

  .settings-view :global(.action-hint) {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    margin: 0;
    line-height: 1.5;
  }
  .settings-view :global(.action-hint strong) {
    color: var(--text-primary);
    font-weight: 600;
  }

  .settings-view :global(.card code) {
    font-family: monospace;
    background: var(--bg-base);
    border-radius: 3px;
    padding: 1px 4px;
    font-size: var(--fs-xs);
    color: var(--accent);
  }

  .settings-view :global(.info-row) {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 0;
    border-bottom: 1px solid var(--border);
  }
  .settings-view :global(.info-row:last-of-type) {
    border-bottom: none;
  }

  .settings-view :global(.info-label) {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    width: 56px;
    flex-shrink: 0;
  }

  .settings-view :global(.info-value) {
    flex: 1;
    font-size: var(--fs-sm);
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    user-select: text;
  }

  .settings-view :global(.mono) {
    font-family: monospace;
  }

  .settings-view :global(.btn-ghost.small) {
    padding: 3px 10px;
    font-size: var(--fs-xs);
    flex-shrink: 0;
    white-space: nowrap;
  }
</style>
