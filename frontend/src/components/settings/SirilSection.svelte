<script lang="ts">
  import { onMount } from "svelte";
  import { CheckSiril, SelectSirilExecutable, SetSirilPath } from "$app";
  import type { SirilInfo } from "../../lib/types";
  import { toast } from "../../lib/toast.svelte";

  interface Props {
    onsirilchange: (info: SirilInfo) => void;
  }

  let { onsirilchange }: Props = $props();

  let sirilInfo = $state<SirilInfo | null>(null);
  let checking = $state(false);
  let saving = $state(false);
  let pathInput = $state("");
  let dirty = $state(false);

  onMount(() => {
    void refresh();
  });

  /** Re-checks Siril; returns the fresh info, or null if the check itself failed. */
  async function refresh(): Promise<SirilInfo | null> {
    checking = true;
    try {
      const info = await CheckSiril();
      sirilInfo = info;
      pathInput = info.executable;
      dirty = false;
      onsirilchange(info);
      return info;
    } catch (e) {
      toast.error(`Couldn't check for Siril: ${String(e)}`);
      return null;
    } finally {
      checking = false;
    }
  }

  async function applyPath(path: string) {
    if (saving) return;
    saving = true;
    try {
      await SetSirilPath(path);
      const info = await refresh();
      if (!info) return;
      if (info.available) toast.success(`Siril path saved — found Siril ${info.version}`);
      else toast.error(`Siril path saved, but Siril wasn't found at "${info.executable}"`);
    } catch (e) {
      toast.error(`Couldn't save the Siril path: ${String(e)}`);
    } finally {
      saving = false;
    }
  }

  function save() {
    if (!dirty) return;
    void applyPath(pathInput.trim());
  }

  function revert() {
    pathInput = sirilInfo?.executable ?? "";
    dirty = false;
  }

  async function browse() {
    let path: string;
    try {
      path = await SelectSirilExecutable();
    } catch (e) {
      toast.error(`Couldn't open the file picker: ${String(e)}`);
      return;
    }
    if (path) await applyPath(path);
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      save();
    } else if (e.key === "Escape" && dirty) {
      e.preventDefault();
      e.stopPropagation();
      revert();
    }
  }

  let isCustom = $derived(
    !!sirilInfo && sirilInfo.executable !== "siril" && sirilInfo.executable !== "",
  );
</script>

<section class="card">
  <h2 class="section-title">Siril</h2>
  <p class="section-desc">
    Siril is used for astrophotography processing. Eirin can open files directly in Siril from the
    right-click menu and uses its command line for frame analysis and stacking.
  </p>

  <div class="siril-status" role="status">
    {#if checking || !sirilInfo}
      <span class="status-dot"></span>
      <span class="status-text">Checking for Siril…</span>
    {:else if sirilInfo.available}
      <span class="status-dot ok"></span>
      <span class="status-text ok">Found: Siril {sirilInfo.version}</span>
    {:else}
      <span class="status-dot err"></span>
      <span class="status-text err">Not found</span>
    {/if}
    <button
      class="btn-ghost small"
      onclick={() => refresh()}
      disabled={checking}
      title="Check again"
      aria-label="Check for Siril again">↺ Re-check</button
    >
  </div>

  <div class="path-row">
    <input
      class="path-input"
      type="text"
      bind:value={pathInput}
      oninput={() => (dirty = true)}
      onkeydown={onKeydown}
      onblur={save}
      placeholder="siril"
      spellcheck="false"
      aria-label="Siril executable"
      readonly={saving}
    />
    <button class="btn-secondary" onclick={browse} disabled={saving}>Browse…</button>
  </div>

  {#if isCustom && !dirty}
    <button class="btn-ghost small reset-btn" onclick={() => applyPath("")} disabled={saving}>
      Reset to default
    </button>
  {/if}

  <p class="action-hint">
    Press <kbd>Enter</kbd> to save (or just click away), <kbd>Esc</kbd> to undo. Leave blank or set
    to <code>siril</code> to use the system PATH; use Browse to locate a custom binary.
  </p>
</section>

<style>
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
    background: var(--text-dim);
  }
  .status-dot.ok {
    background: var(--success);
  }
  .status-dot.err {
    background: var(--danger);
  }

  .status-text {
    flex: 1;
    font-size: var(--fs-sm);
    color: var(--text-secondary);
  }
  .status-text.ok {
    color: var(--success);
  }
  .status-text.err {
    color: var(--danger);
  }

  .reset-btn {
    align-self: flex-start;
  }

  kbd {
    font-family: monospace;
    font-size: var(--fs-xs);
    border: 1px solid var(--border);
    border-radius: 3px;
    padding: 0 4px;
    color: var(--text-primary);
  }
</style>
