<script lang="ts">
  import type * as app from "$models/app";
  import { OpenFolder, OpenProjectInSiril } from "$app";
  import { attempt, toast } from "../../lib/toast.svelte";
  import PathChip from "./PathChip.svelte";

  interface Props {
    project: app.Project;
    sirilAvailable: boolean;
    onremove: () => void;
  }
  let { project, sirilAvailable, onremove }: Props = $props();

  let launching = $state(false);

  async function openInSiril() {
    if (launching || !sirilAvailable) return;
    launching = true;
    toast.info("Opening project in Siril…");
    try {
      await OpenProjectInSiril(project.folder);
      // Siril takes a moment to show its window; keep the feedback up briefly.
      await new Promise((r) => setTimeout(r, 1500));
    } catch (e) {
      toast.error(`Couldn't open Siril: ${String(e)}`);
    } finally {
      launching = false;
    }
  }

  function openFolder() {
    void attempt(() => OpenFolder(project.folder), "Couldn't open the project folder");
  }
</script>

<div class="detail-header">
  <div class="detail-meta">
    <h2 class="detail-name">{project.name}</h2>
    {#if project.description}<p class="detail-desc">{project.description}</p>{/if}
    <PathChip path={project.folder} />
  </div>
  <div class="detail-actions">
    <button
      class="btn-secondary"
      onclick={openFolder}
      title="Show the project folder in your file manager">Open folder</button
    >
    <span
      title={sirilAvailable
        ? "Launch Siril in this project's folder"
        : "Siril not found — configure it in Settings"}
    >
      <button class="btn-primary" disabled={!sirilAvailable || launching} onclick={openInSiril}>
        {launching ? "Launching…" : "Open in Siril"}
      </button>
    </span>
    <button
      class="btn-ghost danger"
      onclick={onremove}
      title="Remove from Eirin's project list — the folder stays on disk">Remove…</button
    >
  </div>
</div>

<style>
  .detail-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: 16px 24px 12px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-panel);
    flex-shrink: 0;
    gap: 16px;
  }
  .detail-meta {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  .detail-name {
    font-size: var(--fs-xl);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }
  .detail-desc {
    font-size: var(--fs-md);
    color: var(--text-secondary);
    margin: 0;
  }
  .detail-actions {
    display: flex;
    gap: 8px;
    flex-shrink: 0;
    align-items: center;
  }
  .detail-actions .btn-secondary {
    margin-right: 0;
  }
</style>
