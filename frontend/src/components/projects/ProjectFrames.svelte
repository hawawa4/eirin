<script lang="ts">
  import { SvelteSet } from "svelte/reactivity";
  import type * as app from "$models/app";
  import {
    sectionsByType,
    PROJECT_TYPE_LABEL,
    type ProjectFrameType,
  } from "../../lib/projects/frames";
  import FrameTable from "../FrameTable.svelte";

  interface Props {
    projectId: number;
    frames: app.LibraryFrame[];
    loading: boolean;
    error: string;
    onretry: () => void;
    onadd: (type: ProjectFrameType) => void;
    onremove: (paths: string[]) => Promise<void>;
  }
  let { projectId, frames, loading, error, onretry, onadd, onremove }: Props = $props();

  let sections = $derived(sectionsByType(frames));
  const collapsed = new SvelteSet<string>();

  function toggle(type: string) {
    if (collapsed.has(type)) collapsed.delete(type);
    else collapsed.add(type);
  }
</script>

<section class="frames">
  <div class="frames-hdr">
    <span class="frames-title">Frames</span>
    {#if !error}<span class="count">{frames.length}</span>{/if}
    {#if loading && frames.length > 0}<span class="refreshing">Refreshing…</span>{/if}
    <span class="spacer"></span>
    <button class="btn-secondary small" onclick={() => onadd("light")}>Add frames…</button>
  </div>

  {#if error}
    <div class="state error" role="alert">
      <span>Couldn't read the project's frames: {error}</span>
      <button class="btn-secondary small" onclick={onretry}>Retry</button>
    </div>
  {:else if loading && frames.length === 0}
    <div class="state">Loading frames…</div>
  {:else}
    {#each sections as s (s.type)}
      {@const open = !collapsed.has(s.type)}
      <div class="type-section">
        <button class="type-hdr" aria-expanded={open} onclick={() => toggle(s.type)}>
          <span class="chevron" aria-hidden="true">{open ? "▾" : "▸"}</span>
          <span class="type-title">{s.label}</span>
          <span class="count">{s.frames.length}</span>
        </button>
        {#if open}
          {#if s.frames.length === 0}
            <p class="type-empty">
              No {s.type in PROJECT_TYPE_LABEL
                ? PROJECT_TYPE_LABEL[s.type as ProjectFrameType].one
                : ""} frames yet.
              {#if s.type in PROJECT_TYPE_LABEL}
                <button class="link-btn" onclick={() => onadd(s.type as ProjectFrameType)}
                  >Add {s.label.toLowerCase()}…</button
                >
              {/if}
            </p>
          {:else}
            <div class="table-wrap">
              <FrameTable
                frames={s.frames}
                {onremove}
                removeDescription="The frames are removed from this project only. The files in your library are not touched."
                hiddenColumns={["frameType"]}
                resetKey={projectId}
                scroll={false}
              />
            </div>
          {/if}
        {/if}
      </div>
    {/each}
  {/if}
</section>

<style>
  .frames {
    border-bottom: 1px solid var(--border);
  }
  .frames-hdr {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 24px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
  }
  .frames-title,
  .type-title {
    font-size: var(--fs-sm);
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .count {
    font-size: var(--fs-sm);
    color: var(--accent);
    font-variant-numeric: tabular-nums;
  }
  .refreshing {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
  .spacer {
    flex: 1;
  }
  .small {
    font-size: var(--fs-xs);
    padding: 3px 10px;
    margin-right: 0;
  }

  .state {
    padding: 16px 24px;
    font-size: var(--fs-md);
    color: var(--text-secondary);
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .state.error {
    color: var(--danger);
  }

  .type-section + .type-section {
    border-top: 1px solid var(--border);
  }
  .type-hdr {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 24px;
    background: var(--bg-base);
    border: none;
    cursor: pointer;
    text-align: left;
    font: inherit;
  }
  .type-hdr:hover {
    background: var(--bg-row-hover);
  }
  .chevron {
    width: 12px;
    color: var(--text-secondary);
  }
  .type-empty {
    margin: 0;
    padding: 4px 24px 12px 44px;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .link-btn {
    background: none;
    border: none;
    padding: 0;
    font: inherit;
    color: var(--accent);
    cursor: pointer;
    text-decoration: underline;
  }
  .table-wrap {
    display: flex;
    flex-direction: column;
    border-top: 1px solid var(--border);
  }
</style>
