<script lang="ts">
  import { onMount } from "svelte";
  import {
    ListDirectoryEnriched,
    RejectFile,
    UnrejectFile,
    HardDeleteFile,
    OpenWithSiril,
  } from "$app";
  import type * as app from "$models/app";
  import type { ColumnDef, CtxEntry, CtxMenuState, IndexProgress } from "../../lib/types";
  import { isFits } from "../../lib/utils";
  import { attempt, toast } from "../../lib/toast.svelte";
  import { isModalOpen, isTypingTarget } from "../../lib/keys";
  import { openFolder, revealPath } from "../../lib/shell/fileActions";
  import { copyPath } from "../../lib/clipboard";
  import NavToolbar from "../NavToolbar.svelte";
  import FileList from "../FileList.svelte";
  import PreviewPane from "../PreviewPane.svelte";
  import SplitPane from "../SplitPane.svelte";
  import ContextMenu from "../ContextMenu.svelte";
  import HardDeleteModal from "../HardDeleteModal.svelte";
  import StatusFooter from "../StatusFooter.svelte";
  import FolderMenu, { type FolderMenuItem } from "./FolderMenu.svelte";

  interface Props {
    rootFolder: string;
    columns: ColumnDef[];
    sirilAvailable: boolean;
    desktopMode: boolean;
    /** True while the Browse tab is visible (keyboard shortcuts only apply then). */
    active: boolean;
    indexRunning: boolean;
    indexProgress: IndexProgress | null;
    onsavecolumns: () => void;
    onscan: () => void;
  }

  let {
    rootFolder,
    columns,
    sirilAvailable,
    desktopMode,
    active,
    indexRunning,
    indexProgress,
    onsavecolumns,
    onscan,
  }: Props = $props();

  // ── Directory listing ─────────────────────────────────────────────────────
  let currentPath = $state("");
  let pathHistory = $state<string[]>([]);
  let files = $state<app.EnrichedFileEntry[]>([]);
  let error = $state("");
  let loading = $state(false);

  // ── Preview / layout ──────────────────────────────────────────────────────
  let selectedEntry = $state<app.EnrichedFileEntry | null>(null);
  let collapsed = $state(false);

  // ── Menus / modals ────────────────────────────────────────────────────────
  let ctxMenu = $state<CtxMenuState | null>(null);
  let folderMenu = $state<{ x: number; y: number; items: FolderMenuItem[] } | null>(null);
  let confirmDel = $state<{ path: string; name: string } | null>(null);

  // ── Footer counts ─────────────────────────────────────────────────────────
  let filteredCount = $state(0);
  let uncachedCount = $derived(
    files.filter((f) => !f.isDir && isFits(f.name) && !f.hasMeta).length,
  );

  onMount(() => {
    if (rootFolder) void loadDirectory(rootFolder);
  });

  /** Re-lists the current folder (e.g. after a library scan). */
  export function reload(): void {
    if (currentPath) void loadDirectory(currentPath, true);
  }

  async function loadDirectory(path: string, keepSelection = false) {
    loading = !keepSelection;
    error = "";
    try {
      const result = await ListDirectoryEnriched(path);
      files = (result || []).slice().sort((a, b) => {
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
        return a.name.localeCompare(b.name);
      });
      currentPath = path;
      if (keepSelection && selectedEntry) {
        selectedEntry = files.find((f) => f.path === selectedEntry!.path) ?? null;
      }
    } catch (e) {
      error = String(e);
      files = [];
    } finally {
      loading = false;
    }
  }

  async function navigateBack() {
    if (pathHistory.length === 0) return;
    const prev = pathHistory[pathHistory.length - 1];
    pathHistory = pathHistory.slice(0, -1);
    selectedEntry = null;
    await loadDirectory(prev);
  }

  async function onRowClick(entry: app.EnrichedFileEntry) {
    if (entry.isDir) {
      selectedEntry = null;
      pathHistory = [...pathHistory, currentPath];
      await loadDirectory(entry.path);
    } else if (isFits(entry.name)) {
      // Clicking the previewed row again closes the preview.
      selectedEntry = selectedEntry?.path === entry.path ? null : entry;
    }
  }

  // Esc closes the preview (menus/modals handle their own Escape first).
  function onWindowKeydown(e: KeyboardEvent) {
    if (e.key !== "Escape" || !active || !selectedEntry) return;
    if (e.defaultPrevented || isTypingTarget(e) || isModalOpen()) return;
    e.preventDefault();
    selectedEntry = null;
  }

  // ── Context menus ─────────────────────────────────────────────────────────
  function onContextMenu(x: number, y: number, entry: app.EnrichedFileEntry) {
    ctxMenu = null;
    folderMenu = null;
    if (entry.isDir) {
      const items: FolderMenuItem[] = [];
      if (desktopMode) items.push({ label: "Show in folder", run: () => revealPath(entry.path) });
      items.push({ label: "Copy path", run: () => copyPath(entry.path) });
      folderMenu = { x, y, items };
      return;
    }
    ctxMenu = {
      x,
      y,
      entry: {
        path: entry.path,
        name: entry.name,
        isRejected: entry.isRejected,
        frameType: "",
      },
      sirilAvailable,
      selectionCount: 1,
    };
  }

  function setRejected(path: string, isRejected: boolean) {
    files = files.map((f) => (f.path === path ? { ...f, isRejected } : f));
    if (selectedEntry?.path === path) selectedEntry = { ...selectedEntry, isRejected };
  }

  async function rejectFile(entry: CtxEntry) {
    ctxMenu = null;
    try {
      await RejectFile(entry.path);
    } catch (e) {
      toast.error(`Couldn't reject ${entry.name}: ${String(e)}`);
      return;
    }
    setRejected(entry.path, true);
    toast.success(`Rejected ${entry.name}`, {
      action: { label: "Undo", run: () => restoreFile(entry) },
    });
  }

  async function restoreFile(entry: CtxEntry) {
    ctxMenu = null;
    try {
      await UnrejectFile(entry.path);
    } catch (e) {
      toast.error(`Couldn't restore ${entry.name}: ${String(e)}`);
      return;
    }
    setRejected(entry.path, false);
  }

  function openHardDeleteConfirm(entry: CtxEntry) {
    ctxMenu = null;
    confirmDel = { path: entry.path, name: entry.name };
  }

  async function doHardDelete() {
    if (!confirmDel) return;
    const { path, name } = confirmDel;
    confirmDel = null;
    try {
      await HardDeleteFile(path);
    } catch (e) {
      toast.error(`Couldn't delete ${name}: ${String(e)}`);
      return;
    }
    files = files.filter((f) => f.path !== path);
    if (selectedEntry?.path === path) selectedEntry = null;
    toast.success(`Deleted ${name}`);
  }

  async function openWithSiril(entry: CtxEntry) {
    ctxMenu = null;
    await attempt(() => OpenWithSiril(entry.path), "Couldn't open Siril");
  }
</script>

<svelte:window onkeydown={onWindowKeydown} />

<div class="browse-view">
  <NavToolbar
    {currentPath}
    canGoBack={pathHistory.length > 0}
    onnavigateBack={navigateBack}
    onreveal={desktopMode ? () => openFolder(currentPath) : undefined}
  />

  <SplitPane showSecondary={!!selectedEntry} collapsedLabel="file list" bind:collapsed>
    {#snippet list()}
      <FileList
        {files}
        {selectedEntry}
        {columns}
        {error}
        {loading}
        dirKey={currentPath}
        onfileclick={onRowClick}
        oncontextmenu={onContextMenu}
        {onsavecolumns}
        onfilteredcountchange={(n) => {
          filteredCount = n;
        }}
      />
    {/snippet}
    {#snippet secondary()}
      {#if selectedEntry}
        <PreviewPane
          entry={selectedEntry}
          onclose={() => {
            selectedEntry = null;
          }}
        />
      {/if}
    {/snippet}
  </SplitPane>

  <StatusFooter
    totalCount={files.length}
    {filteredCount}
    {uncachedCount}
    {indexRunning}
    {indexProgress}
    {rootFolder}
    {onscan}
  />
</div>

{#if ctxMenu}
  <ContextMenu
    menu={ctxMenu}
    onclose={() => {
      ctxMenu = null;
    }}
    onreject={rejectFile}
    onrestore={restoreFile}
    onharddelete={openHardDeleteConfirm}
    onopensiril={openWithSiril}
    onreveal={desktopMode ? (e) => revealPath(e.path) : undefined}
    oncopypath={(e) => copyPath(e.path)}
  />
{/if}

{#if folderMenu}
  <FolderMenu
    x={folderMenu.x}
    y={folderMenu.y}
    items={folderMenu.items}
    onclose={() => {
      folderMenu = null;
    }}
  />
{/if}

{#if confirmDel}
  <HardDeleteModal
    target={confirmDel}
    onconfirm={doHardDelete}
    oncancel={() => {
      confirmDel = null;
    }}
  />
{/if}

<style>
  .browse-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
  }
</style>
