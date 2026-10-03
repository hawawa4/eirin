<script lang="ts">
  import { tick, untrack } from "svelte";
  import { SvelteMap, SvelteSet } from "svelte/reactivity";
  import {
    GetLibraryFrames,
    SetFrameType,
    RejectFile,
    UnrejectFile,
    HardDeleteFile,
    BatchRejectFiles,
    BatchUnrejectFiles,
    BatchHardDeleteFiles,
    OpenWithSiril,
    AnalyzeFrames,
    CancelAnalysis,
    RevealPath,
  } from "$app";
  import { Events } from "@wailsio/runtime";
  import type * as app from "$models/app";
  import type {
    AnalysisProgress,
    ColumnDef,
    CtxEntry,
    CtxMenuState,
    FrameType,
    LibraryGroupBy,
    Project,
  } from "../lib/types";
  import { libraryFrameToEntry, plural } from "../lib/utils";
  import { copyPaths } from "../lib/clipboard";
  import { makeColumnManager } from "../lib/columnManager";
  import { ui } from "../lib/uiState.svelte";
  import { toast, attempt } from "../lib/toast.svelte";
  import { isModalOpen, isTypingTarget } from "../lib/keys";
  import {
    anyColFilterActive,
    distinctValues,
    matchesColFilters,
    matchesSearch,
    type ColFilters,
  } from "../lib/library/filters";
  import {
    buildGroups,
    cursorAfterRemoval,
    groupKeyFor,
    indexByPath,
    positionsByPath,
    sortFrames,
    toGroupBy,
    type LibGroup,
  } from "../lib/library/groups";
  import { flattenGroups, itemOffsets, visibleRange, type TableItem } from "../lib/library/virtual";
  import { forgetPreview } from "../lib/library/previewCache";
  import { planBlink, type BlinkPlan } from "../lib/library/blink";
  import { isProjectFrameType, suggestProjectName } from "../lib/projects/frames";
  import ContextMenu from "./ContextMenu.svelte";
  import HardDeleteModal from "./HardDeleteModal.svelte";
  import BlinkModal from "./BlinkModal.svelte";
  import SplitPane from "./SplitPane.svelte";
  import PreviewPane from "./PreviewPane.svelte";
  import LibraryToolbar from "./library/LibraryToolbar.svelte";
  import SelectionBar from "./library/SelectionBar.svelte";
  import LibraryTableHead from "./library/LibraryTableHead.svelte";
  import GroupHeaderRow from "./library/GroupHeaderRow.svelte";
  import FrameRow from "./library/FrameRow.svelte";
  import TypeFilterPopup from "./library/TypeFilterPopup.svelte";
  import ValuePickerPopup from "./library/ValuePickerPopup.svelte";
  import ShortcutsPopover from "./library/ShortcutsPopover.svelte";
  import CreateProjectModal from "./library/CreateProjectModal.svelte";
  import RenameModal from "./library/RenameModal.svelte";
  import EditMetaModal, { type FrameMetaEdit } from "./library/EditMetaModal.svelte";
  import SuggestRejectsModal from "./library/SuggestRejectsModal.svelte";

  interface Props {
    rootFolder: string;
    columns: ColumnDef[];
    sirilAvailable: boolean;
    onsavecolumns: () => void;
    oncreateproject?: (project: Project) => void;
    /** True while this view's tab is visible; keyboard shortcuts are off otherwise. */
    active?: boolean;
    /** Request a library (re)scan/index. */
    onscan?: () => void;
    /** True while a library index build is running. */
    indexRunning?: boolean;
  }

  let {
    rootFolder,
    columns,
    sirilAvailable,
    onsavecolumns,
    oncreateproject,
    active = true,
    onscan,
    indexRunning = false,
  }: Props = $props();

  const SEARCH_DEBOUNCE_MS = 150;
  /** Extra rows rendered above/below the viewport, in px. */
  const OVERSCAN_PX = 600;

  function baseName(path: string) {
    return path.split(/[\\/]/).pop() ?? path;
  }

  // ── Data ─────────────────────────────────────────────────────────────────
  let frames = $state.raw<app.LibraryFrame[]>([]);
  let loadedOnce = $state(false);
  /** First load for the current root (full spinner). */
  let loading = $state(false);
  /** Background reload — the table stays visible. */
  let refreshing = $state(false);
  let error = $state("");
  let reloadQueued = false;

  let frameByPath = $derived(indexByPath(frames));

  // ── View state ───────────────────────────────────────────────────────────
  let showRejected = $state(false);
  let groupBy = $derived(toGroupBy(ui.libraryGroupBy));
  let sort = $derived(ui.librarySort);

  let searchInput = $state("");
  let search = $state("");
  $effect(() => {
    const v = searchInput.trim();
    if (!v) {
      search = "";
      return;
    }
    const t = setTimeout(() => (search = v), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  });

  let colFilters = $state<ColFilters>({});
  let typeFilterPos = $state<{ x: number; y: number } | null>(null);
  let valuePicker = $state<{ colId: string; x: number; y: number; width: number } | null>(null);
  let showShortcuts = $state(false);

  let colFiltersActive = $derived(anyColFilterActive(colFilters));
  let filtering = $derived(!!search || colFiltersActive);

  function setGroupBy(g: LibraryGroupBy) {
    ui.libraryGroupBy = g;
  }

  function toggleSort(colId: string) {
    const s = ui.librarySort;
    ui.librarySort =
      s?.col === colId
        ? { col: colId, dir: s.dir === "asc" ? "desc" : "asc" }
        : { col: colId, dir: "asc" };
  }

  function switchTab(rejected: boolean) {
    if (showRejected === rejected) return;
    showRejected = rejected;
    selectedPaths.clear();
    previewPath = null;
    anchorPath = null;
  }

  // ── Column filters ────────────────────────────────────────────────────────
  function clearColFilters() {
    colFilters = {};
    typeFilterPos = null;
    valuePicker = null;
  }

  function clearAllFilters() {
    clearColFilters();
    searchInput = "";
    search = "";
  }

  function setTextFilter(colId: string, text: string) {
    colFilters = {
      ...colFilters,
      [colId]: { ...colFilters[colId], text: text || undefined, exact: false },
    };
  }

  /** A value picked from a column's list matches exactly; null clears the filter. */
  function pickValue(colId: string, value: string | null) {
    colFilters = {
      ...colFilters,
      [colId]: { ...colFilters[colId], text: value ?? undefined, exact: value !== null },
    };
    valuePicker = null;
  }

  function toggleValuePicker(colId: string, rect: DOMRect) {
    valuePicker =
      valuePicker?.colId === colId
        ? null
        : { colId, x: rect.left, y: rect.bottom + 2, width: rect.width };
  }

  function setNumFilter(colId: string, val: number | null) {
    const op = colFilters[colId]?.numOp ?? "<";
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], numOp: op, numVal: val } };
  }

  function toggleNumOp(colId: string) {
    const current = colFilters[colId]?.numOp ?? "<";
    const numOp: "<" | ">" = current === "<" ? ">" : "<";
    colFilters = { ...colFilters, [colId]: { ...colFilters[colId], numOp } };
  }

  function toggleTypeFilter(type: FrameType) {
    const current = colFilters.frameType?.types ?? [];
    const types = current.includes(type) ? current.filter((t) => t !== type) : [...current, type];
    colFilters = { ...colFilters, frameType: { ...colFilters.frameType, types } };
  }

  // ── Columns ───────────────────────────────────────────────────────────────
  let dragOverIndex = $state(-1);
  const colMgr = makeColumnManager(
    () => columns,
    (i) => {
      dragOverIndex = i;
    },
    () => onsavecolumns(),
  );

  let visibleColumns = $derived(
    [...columns].filter((c) => c.visible).sort((a, b) => a.order - b.order),
  );
  let totalColWidth = $derived(visibleColumns.reduce((s, c) => s + c.width, 0));

  function toggleColumn(colId: string) {
    const col = columns.find((c) => c.id === colId);
    if (!col) return;
    col.visible = !col.visible;
    onsavecolumns();
  }

  // ── Derived rows ──────────────────────────────────────────────────────────
  let rejectedCount = $derived(frames.reduce((n, f) => n + (f.isRejected ? 1 : 0), 0));
  let tabFrames = $derived(frames.filter((f) => f.isRejected === showRejected));
  let filtered = $derived(
    filtering
      ? tabFrames.filter((f) => matchesSearch(f, search) && matchesColFilters(f, colFilters))
      : tabFrames,
  );
  let sorted = $derived(sortFrames(filtered, sort));
  let pickerOptions = $derived(
    valuePicker ? distinctValues(tabFrames, valuePicker.colId, search, colFilters) : [],
  );
  let groups = $derived<LibGroup[]>(buildGroups(sorted, groupBy));
  /** Frames in the view that can go into a project (no stacks or processed images). */
  let projectFrames = $derived(sorted.filter((f) => isProjectFrameType(f.frameType)));

  // ── Group expansion ───────────────────────────────────────────────────────
  // Explicit choices only; without one, a group is expanded while filtering (so
  // matches are visible) or when there are ≤ 3 groups. Choices made while filtering
  // are temporary and dropped when the filters clear, restoring the manual set.
  const manualExpanded = new SvelteMap<string, boolean>();
  const filterExpanded = new SvelteMap<string, boolean>();

  function isExpanded(key: string): boolean {
    const choice = (filtering ? filterExpanded : manualExpanded).get(key);
    if (choice !== undefined) return choice;
    return filtering || groups.length <= 3;
  }

  $effect(() => {
    if (!filtering) untrack(() => filterExpanded.clear());
  });

  $effect(() => {
    void groupBy;
    untrack(() => {
      manualExpanded.clear();
    });
  });

  function toggleGroup(key: string) {
    (filtering ? filterExpanded : manualExpanded).set(key, !isExpanded(key));
  }

  function setAllExpanded(expanded: boolean) {
    const m = filtering ? filterExpanded : manualExpanded;
    for (const g of groups) m.set(g.key, expanded);
  }

  /** Rows in display order (expanded groups only, ignoring render chunking). */
  let visibleRows = $derived(groups.flatMap((g) => (isExpanded(g.key) ? g.frames : [])));
  let visiblePos = $derived(positionsByPath(visibleRows));

  /** Makes sure `path`'s group is expanded so the row exists in the table. */
  function ensureRowRendered(path: string) {
    const f = frameByPath.get(path);
    if (!f) return;
    const key = groupKeyFor(f, groupBy);
    if (!isExpanded(key)) (filtering ? filterExpanded : manualExpanded).set(key, true);
  }

  // ── Virtualized table body ────────────────────────────────────────────────
  // Only rows intersecting the viewport are in the DOM, so a filter matching tens
  // of thousands of frames (all groups auto-expanded) stays cheap to render.
  // Row heights are measured from the DOM (they scale with the UI font size).
  let rowH = $state(34);
  let groupH = $state(36);
  let headH = $state(0);
  let scrollTop = $state(0);
  let viewportH = $state(600);

  let items = $derived<TableItem[]>(flattenGroups(groups, isExpanded));
  let itemPos = $derived(new Map(items.map((it, i) => [it.key, i])));
  let offsets = $derived(itemOffsets(items, rowH, groupH));
  let range = $derived(
    visibleRange(offsets, scrollTop - OVERSCAN_PX, scrollTop + viewportH - headH + OVERSCAN_PX),
  );
  let windowItems = $derived(items.slice(range.start, range.end));
  let padTop = $derived(offsets[range.start]);
  let padBottom = $derived(offsets[items.length] - offsets[range.end]);

  let scrollFrame = 0;
  function onTableScroll() {
    if (scrollFrame) return;
    scrollFrame = requestAnimationFrame(() => {
      scrollFrame = 0;
      if (tableWrap) scrollTop = tableWrap.scrollTop;
    });
  }

  $effect(() => {
    const wrap = tableWrap;
    if (!wrap) return;
    const ro = new ResizeObserver(() => {
      viewportH = wrap.clientHeight;
      scrollTop = wrap.scrollTop;
      measureRows();
    });
    ro.observe(wrap);
    return () => {
      ro.disconnect();
      cancelAnimationFrame(scrollFrame);
      scrollFrame = 0;
    };
  });

  /** Row pitch = distance to the next row's top (exact, includes borders). */
  function pitchOf(row: Element | null): number | null {
    const next = row?.nextElementSibling;
    if (!row || !next) return null;
    const p = next.getBoundingClientRect().top - row.getBoundingClientRect().top;
    return p > 0 ? p : null;
  }

  function measureRows() {
    const wrap = tableWrap;
    if (!wrap) return;
    const head = wrap.querySelector("thead");
    if (head) headH = head.getBoundingClientRect().height;
    const r = pitchOf(wrap.querySelector("tbody tr.frame-row"));
    if (r !== null && Math.abs(r - rowH) > 0.01) rowH = r;
    const g = pitchOf(wrap.querySelector("tbody tr.group-header-row"));
    if (g !== null && Math.abs(g - groupH) > 0.01) groupH = g;
  }

  // A new query starts at the top of its results.
  $effect(() => {
    void search;
    void colFilters;
    void showRejected;
    const wrap = untrack(() => tableWrap);
    if (wrap) wrap.scrollTop = 0;
  });

  // The browser clamps scrollTop when the list shrinks; resync the window to it.
  $effect(() => {
    void items;
    tick().then(() => {
      if (tableWrap) scrollTop = tableWrap.scrollTop;
    });
  });

  // Re-measure after each window render and when sizes can change.
  $effect(() => {
    void windowItems;
    void visibleColumns;
    void ui.uiScale;
    measureRows();
  });

  // ── Selection & preview cursor ───────────────────────────────────────────
  /** Checked rows (multi-selection). */
  const selectedPaths = new SvelteSet<string>();
  /** The previewed row — also the keyboard cursor. */
  let previewPath = $state<string | null>(null);
  /** Start of the next Shift-click range. */
  let anchorPath: string | null = null;
  let collapsed = $state(false);
  let tableWrap = $state<HTMLDivElement | null>(null);

  let previewFrame = $derived(previewPath ? (frameByPath.get(previewPath) ?? null) : null);
  let previewEntry = $derived(previewFrame ? libraryFrameToEntry(previewFrame) : null);
  let cursorIdx = $derived(previewPath ? (visiblePos.get(previewPath) ?? -1) : -1);
  let prevPath = $derived(cursorIdx > 0 ? visibleRows[cursorIdx - 1].nasPath : null);
  let nextPath = $derived(
    cursorIdx !== -1 && cursorIdx < visibleRows.length - 1
      ? visibleRows[cursorIdx + 1].nasPath
      : null,
  );

  async function scrollToPath(path: string) {
    await tick();
    const wrap = tableWrap;
    const i = itemPos.get(path);
    if (!wrap || i === undefined) return;
    const top = offsets[i];
    const bottom = offsets[i + 1];
    const bodyViewH = wrap.clientHeight - headH;
    if (top < wrap.scrollTop) wrap.scrollTop = top;
    else if (bottom > wrap.scrollTop + bodyViewH) wrap.scrollTop = bottom - bodyViewH;
  }

  function setCursor(path: string | null, scroll = true) {
    previewPath = path;
    if (!path) return;
    anchorPath = path;
    ensureRowRendered(path);
    if (scroll) scrollToPath(path);
  }

  function moveCursor(delta: 1 | -1) {
    const rows = visibleRows;
    if (rows.length === 0) return;
    const i = cursorIdx;
    const next =
      i === -1
        ? delta > 0
          ? 0
          : rows.length - 1
        : Math.max(0, Math.min(rows.length - 1, i + delta));
    setCursor(rows[next].nasPath);
  }

  function toggleChecked(path: string) {
    if (selectedPaths.has(path)) selectedPaths.delete(path);
    else selectedPaths.add(path);
  }

  function onRowClick(e: MouseEvent, frame: app.LibraryFrame) {
    const path = frame.nasPath;
    if (e.shiftKey) {
      const from = anchorPath ?? previewPath;
      const a = from ? visiblePos.get(from) : undefined;
      const b = visiblePos.get(path);
      if (a !== undefined && b !== undefined) {
        const [lo, hi] = a < b ? [a, b] : [b, a];
        for (let i = lo; i <= hi; i++) selectedPaths.add(visibleRows[i].nasPath);
      } else {
        selectedPaths.add(path);
      }
      previewPath = path;
      return;
    }
    if (e.ctrlKey || e.metaKey) {
      // Seed with the previewed row so click + Ctrl-click selects both.
      if (selectedPaths.size === 0 && previewPath && previewPath !== path) {
        selectedPaths.add(previewPath);
      }
      toggleChecked(path);
      anchorPath = path;
      previewPath = path;
      return;
    }
    anchorPath = path;
    previewPath = previewPath === path ? null : path;
  }

  function onCheckbox(frame: app.LibraryFrame) {
    toggleChecked(frame.nasPath);
    anchorPath = frame.nasPath;
  }

  function selectAll() {
    for (const f of sorted) selectedPaths.add(f.nasPath);
  }

  function clearSelection() {
    selectedPaths.clear();
  }

  /** Checked rows, or the previewed row when nothing is checked. */
  function targetPaths(): string[] {
    if (selectedPaths.size > 0) return [...selectedPaths];
    return previewPath ? [previewPath] : [];
  }

  // ── Loading ──────────────────────────────────────────────────────────────
  let pendingFocus: string | null = null;

  async function reload(): Promise<void> {
    const root = rootFolder;
    if (!root) return;
    if (loading || refreshing) {
      reloadQueued = true;
      return;
    }
    const initial = !loadedOnce;
    if (initial) loading = true;
    else refreshing = true;
    try {
      const res = (await GetLibraryFrames(root)) ?? [];
      if (root !== rootFolder) return; // root changed mid-flight; a reload is queued
      frames = res;
      loadedOnce = true;
      error = "";
      pruneMissing();
    } catch (e) {
      if (initial) error = String(e);
      else toast.error(`Could not refresh the library: ${String(e)}`);
    } finally {
      loading = false;
      refreshing = false;
      if (reloadQueued) {
        reloadQueued = false;
        reload();
      } else if (pendingFocus && loadedOnce) {
        applyFocus(pendingFocus);
      }
    }
  }

  /** Drops selection / preview / blink entries whose files vanished after a reload. */
  function pruneMissing() {
    for (const p of [...selectedPaths]) if (!frameByPath.has(p)) selectedPaths.delete(p);
    if (previewPath && !frameByPath.has(previewPath)) previewPath = null;
    if (blinkFrames.length) {
      blinkFrames = blinkFrames.flatMap((f) => {
        const fresh = frameByPath.get(f.nasPath);
        return fresh ? [fresh] : [];
      });
      if (blinkFrames.length < 2) showBlink = false;
    }
  }

  export { reload };

  // (Re)load whenever the root changes — including the first mount.
  let loadedRoot: string | null = null;
  $effect(() => {
    const root = rootFolder;
    untrack(() => {
      if (root === loadedRoot) return;
      loadedRoot = root;
      frames = [];
      loadedOnce = false;
      error = "";
      selectedPaths.clear();
      previewPath = null;
      reload();
    });
  });

  // ── Focus a frame (e.g. from the Sky Atlas) ──────────────────────────────
  /** Reveal, select and preview the frame at `nasPath`; queued until frames are loaded. */
  export function focusFile(nasPath: string): void {
    if (!loadedOnce || loading || refreshing) {
      pendingFocus = nasPath;
      return;
    }
    applyFocus(nasPath);
  }

  function applyFocus(nasPath: string) {
    pendingFocus = null;
    const f = frameByPath.get(nasPath);
    if (!f) {
      toast.info(`${baseName(nasPath)} is not in the library index yet — try a scan.`);
      return;
    }
    clearAllFilters();
    if (showRejected !== f.isRejected) switchTab(f.isRejected);
    selectedPaths.clear();
    manualExpanded.set(groupKeyFor(f, groupBy), true);
    setCursor(nasPath);
  }

  // ── Events ───────────────────────────────────────────────────────────────
  let analyzingGroup = $state<string | null>(null);
  let analysisProgress = $state<AnalysisProgress | null>(null);

  $effect(() => {
    const unsubProgress = Events.On("analysis:progress", (event) => {
      analysisProgress = event.data as AnalysisProgress;
    });
    const unsubUpdated = Events.On("library:updated", () => {
      reload();
    });
    return () => {
      unsubProgress();
      unsubUpdated();
    };
  });

  async function analyzeGroup(group: LibGroup, force: boolean) {
    const paths = (force ? group.frames : group.frames.filter((f) => !f.qualityAnalyzed)).map(
      (f) => f.nasPath,
    );
    if (!paths.length) return;
    analyzingGroup = group.key;
    analysisProgress = null;
    const ok = await attempt(async () => {
      await AnalyzeFrames(paths);
      return true;
    }, `Analysis of “${group.label}” failed`);
    const errors = (analysisProgress as AnalysisProgress | null)?.errors ?? 0;
    analyzingGroup = null;
    analysisProgress = null;
    if (ok) {
      if (errors > 0) toast.error(`Analysis finished with ${plural(errors, "error")}`);
      else toast.success(`Analyzed ${plural(paths.length, "frame")} in “${group.label}”`);
    }
    reload();
  }

  function cancelAnalysis() {
    attempt(() => CancelAnalysis(), "Could not cancel the analysis");
  }

  // ── Reject / restore / delete ────────────────────────────────────────────
  /**
   * Sets the rejected flag on `paths` (skipping ones already in that state).
   * Optimistic: the state flips synchronously and is reverted if the backend call
   * fails. Shows a toast with Undo unless `undoable` is false. Resolves true on success.
   */
  async function setRejected(paths: string[], rejected: boolean, undoable = true) {
    const list = paths.filter((p) => {
      const f = frameByPath.get(p);
      return f !== undefined && f.isRejected !== rejected;
    });
    if (list.length === 0) return true;
    const changed = new Set(list);
    const applyFlag = (value: boolean) => {
      const flip = (f: app.LibraryFrame) =>
        changed.has(f.nasPath) ? { ...f, isRejected: value } : f;
      frames = frames.map(flip);
      blinkFrames = blinkFrames.map(flip);
    };
    applyFlag(rejected);
    const ok = await attempt(
      async () => {
        if (list.length === 1) await (rejected ? RejectFile(list[0]) : UnrejectFile(list[0]));
        else await (rejected ? BatchRejectFiles(list) : BatchUnrejectFiles(list));
        return true;
      },
      `Could not ${rejected ? "reject" : "restore"} ${plural(list.length, "frame")}`,
    );
    if (!ok) {
      applyFlag(!rejected);
      if (list.length > 1) reload(); // a batch may have partially applied
      return false;
    }
    if (undoable) {
      toast.success(`${rejected ? "Rejected" : "Restored"} ${plural(list.length, "frame")}`, {
        action: {
          label: "Undo",
          run: async () => {
            await setRejected(list, !rejected, false);
          },
        },
      });
    }
    return true;
  }

  /**
   * Rejects/restores `paths`; rows that leave the current tab are unchecked and the
   * preview cursor advances past them right away (so rapid `x` presses keep culling).
   */
  function applyReject(paths: string[], rejected: boolean): Promise<boolean> {
    if (paths.length === 0) return Promise.resolve(true);
    const leavesTab = rejected !== showRejected;
    const leaving = new Set(paths);
    const next = leavesTab ? cursorAfterRemoval(visibleRows, previewPath, leaving) : previewPath;
    const result = setRejected(paths, rejected); // flips state synchronously
    if (leavesTab) {
      for (const p of paths) selectedPaths.delete(p);
      if (previewPath && leaving.has(previewPath)) setCursor(next);
    }
    return result;
  }

  let confirmDel = $state<{ paths: string[]; name: string } | null>(null);

  function askDelete(paths: string[]) {
    if (paths.length === 0) return;
    confirmDel = {
      paths,
      name:
        paths.length === 1
          ? (frameByPath.get(paths[0])?.fileName ?? baseName(paths[0]))
          : `${paths.length} frames`,
    };
  }

  async function hardDelete(paths: string[]): Promise<boolean> {
    const removed = new Set(paths);
    const next = cursorAfterRemoval(visibleRows, previewPath, removed);
    const ok = await attempt(
      async () => {
        if (paths.length === 1) await HardDeleteFile(paths[0]);
        else await BatchHardDeleteFiles(paths);
        return true;
      },
      `Could not delete ${plural(paths.length, "frame")}`,
    );
    if (!ok) {
      if (paths.length > 1) reload();
      return false;
    }
    frames = frames.filter((f) => !removed.has(f.nasPath));
    blinkFrames = blinkFrames.filter((f) => !removed.has(f.nasPath));
    if (blinkFrames.length < 2) showBlink = false;
    for (const p of paths) {
      selectedPaths.delete(p);
      forgetPreview(p);
    }
    if (previewPath && removed.has(previewPath)) setCursor(next);
    toast.success(`Deleted ${plural(paths.length, "frame")} from disk`);
    return true;
  }

  async function confirmHardDelete() {
    const target = confirmDel;
    confirmDel = null;
    if (target) await hardDelete(target.paths);
  }

  // ── Frame type ───────────────────────────────────────────────────────────
  async function changeFrameType(path: string, newType: string, select?: HTMLSelectElement) {
    const ok = await attempt(async () => {
      await SetFrameType(path, newType);
      return true;
    }, "Could not change the frame type");
    if (!ok) {
      if (select) select.value = frameByPath.get(path)?.frameType ?? select.value;
      return;
    }
    frames = frames.map((f) => (f.nasPath === path ? { ...f, frameType: newType } : f));
  }

  // ── Shell helpers ────────────────────────────────────────────────────────
  function reveal(path: string) {
    attempt(() => RevealPath(path), "Could not show the file in its folder");
  }

  function openWithSiril(path: string) {
    toast.info("Opening in Siril…");
    attempt(() => OpenWithSiril(path), "Could not open the file in Siril");
  }

  // ── Context menu ─────────────────────────────────────────────────────────
  let ctxMenu = $state<CtxMenuState | null>(null);
  let ctxPaths = $state<string[]>([]);

  function openCtxMenu(e: MouseEvent, frame: app.LibraryFrame) {
    e.preventDefault();
    const inSelection = selectedPaths.has(frame.nasPath) && selectedPaths.size > 1;
    const paths = inSelection ? [...selectedPaths] : [frame.nasPath];
    ctxPaths = paths;
    ctxMenu = {
      x: e.clientX,
      y: e.clientY,
      entry: {
        path: frame.nasPath,
        name: frame.fileName,
        isRejected: frame.isRejected,
        frameType: frame.frameType,
      },
      sirilAvailable: sirilAvailable && paths.length === 1,
      selectionCount: paths.length,
    };
  }

  function closeCtx() {
    ctxMenu = null;
  }

  // ── Modals ───────────────────────────────────────────────────────────────
  let renameTarget = $state<{ path: string; name: string } | null>(null);
  let metaTarget = $state<{ path: string; initial: FrameMetaEdit } | null>(null);
  let cpFrames = $state<app.LibraryFrame[] | null>(null);
  let showSuggest = $state(false);

  function onRenamed(oldPath: string, newPath: string, newName: string) {
    frames = frames.map((f) =>
      f.nasPath === oldPath ? { ...f, nasPath: newPath, fileName: newName } : f,
    );
    forgetPreview(oldPath);
    if (selectedPaths.delete(oldPath)) selectedPaths.add(newPath);
    if (previewPath === oldPath) previewPath = newPath;
    if (anchorPath === oldPath) anchorPath = newPath;
    toast.success(`Renamed to ${newName}`);
  }

  function editMeta(entry: CtxEntry) {
    const f = frameByPath.get(entry.path);
    metaTarget = {
      path: entry.path,
      initial: {
        object: f?.object ?? "",
        telescope: f?.telescope ?? "",
        filter: f?.filter ?? "",
        dateObs: f?.dateObs ?? "",
      },
    };
  }

  function onMetaSaved(path: string, m: FrameMetaEdit) {
    frames = frames.map((f) =>
      f.nasPath === path
        ? {
            ...f,
            object: m.object || f.object,
            telescope: m.telescope || f.telescope,
            filter: m.filter || f.filter,
            dateObs: m.dateObs || f.dateObs,
          }
        : f,
    );
    toast.success("Metadata saved");
  }

  function openCreateProject(paths: string[]) {
    cpFrames = paths.flatMap((p) => {
      const f = frameByPath.get(p);
      return f ? [f] : [];
    });
  }

  function createProjectFromView() {
    cpFrames = projectFrames;
  }

  // ── Blink ────────────────────────────────────────────────────────────────
  let blinkFrames = $state.raw<app.LibraryFrame[]>([]);
  let blinkStart = $state<string | null>(null);
  let showBlink = $state(false);

  /**
   * 2+ checked frames blink those; otherwise the previewed (or single checked)
   * frame's whole object — same object and type — within the current view.
   */
  let blinkPlan = $derived.by((): BlinkPlan | null => {
    const ordered = groups.flatMap((g) => g.frames);
    let checked: app.LibraryFrame[] = [];
    if (selectedPaths.size >= 2) {
      // Display order first, then any checked frames hidden by the current filters.
      const inView = ordered.filter((f) => selectedPaths.has(f.nasPath));
      const seen = new Set(inView.map((f) => f.nasPath));
      const rest = frames.filter((f) => selectedPaths.has(f.nasPath) && !seen.has(f.nasPath));
      checked = [...inView, ...rest];
    }
    const anchorPath = previewPath ?? (selectedPaths.size === 1 ? [...selectedPaths][0] : null);
    const anchor = anchorPath ? frameByPath.get(anchorPath) : undefined;
    return planBlink(ordered, checked, anchor);
  });

  function openBlink() {
    const plan = blinkPlan;
    if (!plan) {
      toast.info(
        previewPath || selectedPaths.size === 1
          ? "No other frames of this object and type in the current view"
          : "Preview a frame to blink its object, or check 2 or more frames",
      );
      return;
    }
    blinkFrames = plan.frames;
    blinkStart = plan.start;
    showBlink = true;
  }

  // ── Keyboard ─────────────────────────────────────────────────────────────
  function onWindowKeydown(e: KeyboardEvent) {
    if (active === false || e.defaultPrevented) return;
    // A focused row checkbox (after clicking it) shouldn't disable culling keys;
    // Space on it keeps its native toggle.
    const onCheckbox = e.target instanceof HTMLInputElement && e.target.type === "checkbox";
    if ((isTypingTarget(e) && !onCheckbox) || isModalOpen()) return;
    if (onCheckbox && e.key === " ") return;
    if (ctxMenu || showBlink || confirmDel) return;

    const mod = e.ctrlKey || e.metaKey;
    if (mod && !e.altKey && (e.key === "a" || e.key === "A")) {
      e.preventDefault();
      selectAll();
      return;
    }
    if (mod || e.altKey) return;

    switch (e.key) {
      case "ArrowDown":
      case "j":
        e.preventDefault();
        moveCursor(1);
        return;
      case "ArrowUp":
      case "k":
        e.preventDefault();
        moveCursor(-1);
        return;
      case "x":
        e.preventDefault();
        if (!showRejected) applyReject(targetPaths(), true);
        return;
      case "u":
        e.preventDefault();
        applyReject(targetPaths(), false);
        return;
      case "Delete":
        e.preventDefault();
        askDelete(targetPaths());
        return;
      case " ":
        if (!previewPath) return;
        e.preventDefault();
        toggleChecked(previewPath);
        anchorPath = previewPath;
        return;
      case "b":
        e.preventDefault();
        openBlink();
        return;
      case "?":
        e.preventDefault();
        showShortcuts = !showShortcuts;
        return;
      case "Escape":
        if (showShortcuts) showShortcuts = false;
        else if (previewPath) previewPath = null;
        else if (selectedPaths.size > 0) selectedPaths.clear();
        else return;
        e.preventDefault();
        return;
    }
  }
</script>

<svelte:window onkeydown={onWindowKeydown} />

<div class="library-view">
  <LibraryToolbar
    {showRejected}
    {rejectedCount}
    ontab={switchTab}
    {groupBy}
    ongroupby={setGroupBy}
    bind:search={searchInput}
    onexpandall={() => setAllExpanded(true)}
    oncollapseall={() => setAllExpanded(false)}
    sortActive={!!sort}
    onclearsort={() => (ui.librarySort = null)}
    filtersActive={filtering || !!searchInput}
    onclearfilters={clearAllFilters}
    {refreshing}
    visibleCount={sorted.length}
    onselectall={selectAll}
    onsuggest={sirilAvailable ? () => (showSuggest = true) : undefined}
    oncreateproject={oncreateproject ? createProjectFromView : undefined}
    projectFrameCount={projectFrames.length}
    {columns}
    ontogglecolumn={toggleColumn}
    shortcutsOpen={showShortcuts}
    onshortcuts={() => (showShortcuts = !showShortcuts)}
  />

  <div class="popover-anchor">
    {#if showShortcuts}
      <ShortcutsPopover onclose={() => (showShortcuts = false)} />
    {/if}
  </div>

  {#if selectedPaths.size > 0}
    <SelectionBar
      count={selectedPaths.size}
      {showRejected}
      onreject={() => applyReject([...selectedPaths], true)}
      onrestore={() => applyReject([...selectedPaths], false)}
      ondelete={() => askDelete([...selectedPaths])}
      onblink={openBlink}
      blinkLabel={blinkPlan?.label ?? null}
      oncreateproject={oncreateproject ? () => openCreateProject([...selectedPaths]) : undefined}
      onclear={clearSelection}
    />
  {/if}

  <SplitPane showSecondary={!!previewEntry} collapsedLabel="library" bind:collapsed>
    {#snippet list()}
      {#if !loadedOnce && loading}
        <div class="status-row" role="status">
          <span class="spinner" aria-hidden="true"></span>
          <span>Loading library…</span>
        </div>
      {:else if !loadedOnce && error}
        <div class="status-row error" role="alert">
          <p>Could not load the library: {error}</p>
          <button class="btn-secondary" onclick={() => reload()}>Retry</button>
        </div>
      {:else}
        <div class="table-scroll-wrapper" bind:this={tableWrap} onscroll={onTableScroll}>
          <table
            class="lib-table"
            style="width: {Math.max(totalColWidth + 28, 100)}px; min-width: 100%"
          >
            <colgroup>
              <col style="width: 28px" />
              {#each visibleColumns as col (col.id)}
                <col style="width: {col.width}px" />
              {/each}
            </colgroup>
            <LibraryTableHead
              columns={visibleColumns}
              {sort}
              {colFilters}
              {dragOverIndex}
              {colMgr}
              ontogglesort={toggleSort}
              ondragleavecol={(i) => {
                if (dragOverIndex === i) dragOverIndex = -1;
              }}
              ontextfilter={setTextFilter}
              onnumfilter={setNumFilter}
              ontogglenumop={toggleNumOp}
              ontypefilter={(rect) => {
                typeFilterPos = typeFilterPos ? null : { x: rect.left, y: rect.bottom + 2 };
              }}
              onpickvalues={toggleValuePicker}
              pickerCol={valuePicker?.colId ?? null}
            />
            <tbody>
              {#if groups.length === 0}
                <tr class="empty-row">
                  <td colspan={visibleColumns.length + 1}>
                    <div class="empty-msg">
                      {#if frames.length === 0}
                        <p>No indexed frames found.</p>
                        {#if onscan}
                          <button
                            class="btn-primary"
                            onclick={() => onscan?.()}
                            disabled={indexRunning}
                          >
                            {indexRunning ? "Scanning…" : "Scan library"}
                          </button>
                        {:else}
                          <p>Build the index from Settings to catalog your frames.</p>
                        {/if}
                      {:else if filtering}
                        <p>No frames match the current filter.</p>
                        <button class="btn-secondary" onclick={clearAllFilters}>
                          Remove filters
                        </button>
                      {:else if showRejected}
                        <p>No rejected frames.</p>
                      {:else}
                        <p>Every frame is rejected — see the Rejected tab.</p>
                      {/if}
                    </div>
                  </td>
                </tr>
              {/if}
              {#if padTop > 0}
                <tr class="spacer-row" aria-hidden="true">
                  <td colspan={visibleColumns.length + 1} style="height: {padTop}px"></td>
                </tr>
              {/if}
              {#each windowItems as item (item.key)}
                {#if item.kind === "group"}
                  {@const group = item.group}
                  <GroupHeaderRow
                    {group}
                    colspan={visibleColumns.length + 1}
                    expanded={isExpanded(group.key)}
                    ontoggle={() => toggleGroup(group.key)}
                    {sirilAvailable}
                    analyzing={analyzingGroup === group.key}
                    analysisBusy={analyzingGroup !== null}
                    {analysisProgress}
                    onanalyze={(force) => analyzeGroup(group, force)}
                    oncancelanalysis={cancelAnalysis}
                  />
                {:else}
                  {@const frame = item.frame}
                  <FrameRow
                    {frame}
                    columns={visibleColumns}
                    current={previewPath === frame.nasPath}
                    checked={selectedPaths.has(frame.nasPath)}
                    onrowclick={(e) => onRowClick(e, frame)}
                    onctxmenu={(e) => openCtxMenu(e, frame)}
                    oncheck={() => onCheckbox(frame)}
                    onchangetype={(t, el) => changeFrameType(frame.nasPath, t, el)}
                  />
                {/if}
              {/each}
              {#if padBottom > 0}
                <tr class="spacer-row" aria-hidden="true">
                  <td colspan={visibleColumns.length + 1} style="height: {padBottom}px"></td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      {/if}
    {/snippet}
    {#snippet secondary()}
      {#if previewEntry && previewFrame}
        {@const path = previewFrame.nasPath}
        <PreviewPane
          entry={previewEntry}
          qualityFrame={previewFrame}
          onclose={() => (previewPath = null)}
          onprev={prevPath ? () => setCursor(prevPath) : undefined}
          onnext={nextPath ? () => setCursor(nextPath) : undefined}
          onreject={() => applyReject([path], true)}
          onrestore={() => applyReject([path], false)}
          ondelete={() => askDelete([path])}
          onreveal={() => reveal(path)}
          onblink={blinkPlan ? openBlink : undefined}
          blinkTitle={blinkPlan ? `Blink ${blinkPlan.label} (b)` : undefined}
          prefetch={nextPath}
        />
      {/if}
    {/snippet}
  </SplitPane>
</div>

{#if valuePicker}
  {@const cf = colFilters[valuePicker.colId]}
  <ValuePickerPopup
    x={valuePicker.x}
    y={valuePicker.y}
    minWidth={Math.max(valuePicker.width, 180)}
    label={columns.find((c) => c.id === valuePicker?.colId)?.label ?? "Value"}
    options={pickerOptions}
    selected={cf?.exact && cf.text ? cf.text : null}
    onpick={(v) => valuePicker && pickValue(valuePicker.colId, v)}
    onclose={() => (valuePicker = null)}
  />
{/if}

{#if typeFilterPos}
  <TypeFilterPopup
    x={typeFilterPos.x}
    y={typeFilterPos.y}
    selected={colFilters.frameType?.types ?? []}
    ontoggle={toggleTypeFilter}
    onclose={() => (typeFilterPos = null)}
  />
{/if}

{#if ctxMenu}
  <ContextMenu
    menu={ctxMenu}
    onclose={closeCtx}
    onreject={() => {
      closeCtx();
      applyReject(ctxPaths, true);
    }}
    onrestore={() => {
      closeCtx();
      applyReject(ctxPaths, false);
    }}
    onharddelete={() => {
      closeCtx();
      askDelete(ctxPaths);
    }}
    onopensiril={(entry) => {
      closeCtx();
      openWithSiril(entry.path);
    }}
    onchangetype={(entry, t) => {
      closeCtx();
      changeFrameType(entry.path, t);
    }}
    onrename={(entry) => {
      closeCtx();
      renameTarget = { path: entry.path, name: entry.name };
    }}
    oneditmeta={(entry) => {
      closeCtx();
      editMeta(entry);
    }}
    oncreateproject={oncreateproject
      ? () => {
          closeCtx();
          openCreateProject(ctxPaths);
        }
      : undefined}
    onreveal={(entry) => {
      closeCtx();
      reveal(entry.path);
    }}
    oncopypath={() => {
      closeCtx();
      copyPaths(ctxPaths);
    }}
  />
{/if}

{#if cpFrames}
  <CreateProjectModal
    frames={cpFrames}
    suggestedName={suggestProjectName(cpFrames)}
    onclose={() => (cpFrames = null)}
    oncreated={(project) => oncreateproject?.(project)}
  />
{/if}

{#if renameTarget}
  <RenameModal
    path={renameTarget.path}
    name={renameTarget.name}
    onclose={() => (renameTarget = null)}
    onrenamed={onRenamed}
  />
{/if}

{#if metaTarget}
  <EditMetaModal
    path={metaTarget.path}
    initial={metaTarget.initial}
    onclose={() => (metaTarget = null)}
    onsaved={onMetaSaved}
  />
{/if}

{#if showSuggest}
  <SuggestRejectsModal
    {rootFolder}
    onclose={() => (showSuggest = false)}
    onapply={(paths) => applyReject(paths, true)}
  />
{/if}

{#if showBlink && blinkFrames.length >= 2}
  <BlinkModal
    frames={blinkFrames}
    startPath={blinkStart}
    onclose={() => (showBlink = false)}
    onreject={(p) => applyReject([p], true)}
    onrestore={(p) => applyReject([p], false)}
    onharddelete={(p) => hardDelete([p])}
  />
{/if}

{#if confirmDel}
  <HardDeleteModal
    target={confirmDel}
    onconfirm={confirmHardDelete}
    oncancel={() => (confirmDel = null)}
  />
{/if}

<style>
  .library-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
    overflow: hidden;
  }

  .popover-anchor {
    position: relative;
    height: 0;
    z-index: 250;
  }

  /* ── Status ──────────────────────────────────────────────────────────────── */

  .status-row {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    font-size: var(--fs-md);
    color: var(--text-secondary);
  }
  .status-row.error {
    color: var(--danger);
  }
  .spinner {
    display: inline-block;
    width: 28px;
    height: 28px;
    border: 3px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* ── Table ───────────────────────────────────────────────────────────────── */

  .table-scroll-wrapper {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .table-scroll-wrapper::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }
  .table-scroll-wrapper::-webkit-scrollbar-track {
    background: var(--bg-base);
  }
  .table-scroll-wrapper::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 4px;
  }

  .lib-table {
    /* Separate borders keep every row's pitch exact for the virtualized body. */
    border-collapse: separate;
    border-spacing: 0;
    font-size: var(--fs-md);
    table-layout: fixed;
  }

  .empty-row td {
    padding: 36px 0;
    text-align: center;
    color: var(--text-secondary);
    font-size: var(--fs-md);
  }

  .empty-msg {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
  }

  .spacer-row td {
    padding: 0;
    border: 0;
  }
</style>
