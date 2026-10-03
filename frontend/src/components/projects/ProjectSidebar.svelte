<script lang="ts">
  import type * as app from "$models/app";
  import { formatShortDate } from "../../lib/projects/frames";

  interface Props {
    projects: app.Project[];
    selectedId: number | null;
    loading: boolean;
    error: string;
    projectsFolder: string;
    onselect: (p: app.Project) => void;
    oncreate: () => void;
    onretry: () => void;
  }
  let { projects, selectedId, loading, error, projectsFolder, onselect, oncreate, onretry }: Props =
    $props();
</script>

<aside class="project-sidebar">
  <div class="sidebar-header">
    <span class="sidebar-title">Projects</span>
    <span title={projectsFolder ? "New project" : "Set a Projects folder in Settings first"}>
      <button
        class="btn-icon small"
        aria-label="New project"
        disabled={!projectsFolder}
        onclick={oncreate}>+</button
      >
    </span>
  </div>

  {#if !projectsFolder}
    <p class="sidebar-hint">Configure a Projects folder in Settings first.</p>
  {:else if error}
    <div class="sidebar-hint error" role="alert">
      <p>Couldn't load projects: {error}</p>
      <button class="btn-secondary" onclick={onretry}>Retry</button>
    </div>
  {:else if loading && projects.length === 0}
    <p class="sidebar-hint">Loading…</p>
  {:else if projects.length === 0}
    <div class="sidebar-hint">
      <p>No projects yet.</p>
      <button class="btn-secondary" onclick={oncreate}>New project…</button>
    </div>
  {:else}
    <ul class="project-list">
      {#each projects as p (p.id)}
        <li class="project-item" class:active={selectedId === p.id}>
          <button
            class="project-item-btn"
            aria-current={selectedId === p.id ? "true" : undefined}
            onclick={() => onselect(p)}
          >
            <span class="project-name" title={p.name}>{p.name}</span>
            <span class="project-date">{formatShortDate(p.createdAt)}</span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</aside>

<style>
  .project-sidebar {
    width: 230px;
    flex-shrink: 0;
    background: var(--bg-panel);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 12px 8px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .sidebar-title {
    font-size: var(--fs-sm);
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .sidebar-hint {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    padding: 16px 12px;
    line-height: 1.5;
    margin: 0;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  .sidebar-hint p {
    margin: 0;
  }
  .sidebar-hint.error p {
    color: var(--danger);
    word-break: break-word;
  }
  .project-list {
    list-style: none;
    margin: 0;
    padding: 4px 0;
    overflow-y: auto;
    flex: 1;
  }
  .project-item {
    border-left: 3px solid transparent;
    transition: background 0.12s;
  }
  .project-item:hover {
    background: var(--bg-row-hover);
  }
  .project-item.active {
    background: var(--bg-row-hover);
    border-left-color: var(--accent);
  }
  .project-item-btn {
    display: flex;
    flex-direction: column;
    width: 100%;
    padding: 7px 12px;
    cursor: pointer;
    background: none;
    border: none;
    text-align: left;
    color: inherit;
    font: inherit;
  }
  .project-name {
    font-size: var(--fs-md);
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .project-date {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    margin-top: 1px;
  }
</style>
