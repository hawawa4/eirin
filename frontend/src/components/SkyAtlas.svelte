<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { Events } from "@wailsio/runtime";
  import type * as app from "$models/app";
  import { GetAtlasIndex, GetAtlasFrameSize, GetCatalog } from "$app";
  import { formatDec, formatRA } from "../lib/utils";
  import { toast } from "../lib/toast.svelte";
  import { isModalOpen, isTypingTarget } from "../lib/keys";
  import {
    angularSep,
    clampPpd,
    fromTangent,
    panFrom,
    toTangent,
    zoomAnchored,
    type Viewport,
  } from "../lib/atlas/projection";
  import { frameExtentDeg } from "../lib/atlas/footprint";
  import { fitView, type FitResult, type FitTarget } from "../lib/atlas/fit";
  import { drawScene } from "../lib/atlas/draw";
  import { framesNeedingSize, hitTest, type AtlasScene } from "../lib/atlas/scene";
  import { SizeQueue } from "../lib/atlas/sizeQueue";
  import { PreviewCache } from "../lib/atlas/previews";
  import {
    buildObjectGroups,
    isFinalImage,
    objectKey,
    topFrame,
    type ObjectGroup,
  } from "../lib/atlas/objects";
  import { NO_FIX, effectiveRotation, type OrientationFix } from "../lib/atlas/orientation";
  import ObjectBrowser from "./atlas/ObjectBrowser.svelte";
  import AtlasPanel from "./atlas/AtlasPanel.svelte";
  import AtlasTooltip from "./atlas/AtlasTooltip.svelte";
  import AtlasStatus from "./atlas/AtlasStatus.svelte";
  import HelpPopover from "./atlas/HelpPopover.svelte";

  interface Props {
    rootPath: string;
    onframeopen?: (nasPath: string) => void;
    /** True while this view's tab is visible. Keyboard shortcuts and rendering pause otherwise. */
    active?: boolean;
    /** Request a library (re)scan/index — offered when nothing is indexed yet. */
    onscan?: () => void;
  }

  let { rootPath, onframeopen, active = true, onscan }: Props = $props();

  // ── Canvas & camera ───────────────────────────────────────────────────────
  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let container: HTMLElement;
  let canvasW = $state(800);
  let canvasH = $state(600);
  let dpr = $state(1);

  let viewRA = $state(180);
  let viewDec = $state(0);
  let pixPerDeg = $state(12);
  let vp = $derived<Viewport>({ ra: viewRA, dec: viewDec, ppd: pixPerDeg, w: canvasW, h: canvasH });

  // ── Data ──────────────────────────────────────────────────────────────────
  let index = $state.raw<app.AtlasIndexEntry[]>([]);
  let catalog = $state.raw<app.CatalogObject[]>([]);
  let loading = $state(true);
  let loadError = $state("");

  // Filter: null = automatic ("Final images" when there are any, else "All frames").
  let filterMode = $state<"final" | "all" | null>(null);
  let finalCount = $derived(index.filter(isFinalImage).length);
  let showAll = $derived(filterMode ? filterMode === "all" : finalCount === 0);
  let visibleIndex = $derived(showAll ? index : index.filter(isFinalImage));
  let hiddenCount = $derived(index.length - visibleIndex.length);
  let objectGroups = $derived(buildObjectGroups(visibleIndex));

  let showLabels = $state(true);
  let helpOpen = $state(false);

  // ── Selection ─────────────────────────────────────────────────────────────
  // Overlaid frames by path, z-order (last = top = shown in the panel). Paths,
  // not entries, so a library reload keeps the selection.
  let overlayPaths = $state.raw<string[]>([]);
  let visibleByPath = $derived(new Map(visibleIndex.map((e) => [e.nasPath, e])));
  let overlay = $derived(
    overlayPaths
      .map((p) => visibleByPath.get(p))
      .filter((e): e is app.AtlasIndexEntry => e !== undefined),
  );
  let panelEntry = $derived(overlay.length > 0 ? overlay[overlay.length - 1] : null);
  let activeObject = $derived(panelEntry ? objectKey(panelEntry) : null);

  let hoveredEntry = $state.raw<app.AtlasIndexEntry | null>(null);
  let hoverX = $state(0);
  let hoverY = $state(0);
  let focus = $state.raw<{ ra: number; dec: number } | null>(null);
  let fixes = $state.raw<Record<string, OrientationFix>>({});

  // ── Lazy caches (non-reactive; cacheVersion drives redraws) ───────────────
  let cacheVersion = $state(0);
  const sizeQueue = new SizeQueue(
    (p) => GetAtlasFrameSize(p),
    () => cacheVersion++,
    4,
  );
  const previews = new PreviewCache(
    () => cacheVersion++,
    (entry, err) => toast.error(`Couldn't load preview for ${entry.name}: ${String(err)}`),
  );
  let sizesBusy = $derived(cacheVersion >= 0 ? sizeQueue.busy : 0);
  let previewsBusy = $derived(cacheVersion >= 0 ? previews.loadingCount : 0);

  let scene = $derived.by<AtlasScene>(() => {
    const f = fixes;
    return {
      vp,
      frames: visibleIndex,
      overlay,
      hoveredPath: hoveredEntry?.nasPath ?? null,
      catalog,
      showLabels,
      focus,
      size: (p) => sizeQueue.get(p),
      preview: (p) => previews.get(p),
      previewLoading: (p) => previews.isLoading(p),
      rotation: (e) => effectiveRotation(e, f[e.nasPath]),
    };
  });

  // ── Rendering ─────────────────────────────────────────────────────────────
  let redrawPending = false;
  function scheduleRedraw() {
    if (redrawPending) return;
    redrawPending = true;
    requestAnimationFrame(() => {
      redrawPending = false;
      if (!ctx) return;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      drawScene(ctx, scene);
    });
  }

  $effect(() => {
    void scene;
    void cacheVersion;
    void dpr;
    if (active) scheduleRedraw();
  });

  // Previews follow the overlay.
  $effect(() => {
    previews.sync(overlay);
  });

  // Frame sizes for footprints: fetched for frames in view once the camera settles.
  let lazyTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    // Depend on the camera and filter only (not on `scene`, which also changes on hover).
    void vp;
    void visibleIndex;
    void fixes;
    if (!active) return;
    if (lazyTimer) clearTimeout(lazyTimer);
    lazyTimer = setTimeout(() => sizeQueue.request(framesNeedingSize(scene)), 200);
  });

  // ── Camera moves ──────────────────────────────────────────────────────────
  let anim: number | null = null;
  function stopAnim() {
    if (anim !== null) cancelAnimationFrame(anim);
    anim = null;
  }

  function goTo(t: FitResult, animate = true) {
    stopAnim();
    const start = { ra: viewRA, dec: viewDec, ppd: pixPerDeg };
    const off = toTangent(start.ra, start.dec, t.ra, t.dec);
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    if (!animate || reduceMotion || !off || angularSep(start.ra, start.dec, t.ra, t.dec) > 60) {
      viewRA = t.ra;
      viewDec = t.dec;
      pixPerDeg = t.ppd;
      return;
    }
    const t0 = performance.now();
    const dur = 380;
    const step = (now: number) => {
      const k = Math.min(1, (now - t0) / dur);
      const e = 1 - (1 - k) ** 3;
      const [ra, dec] = fromTangent(start.ra, start.dec, off[0] * e, off[1] * e);
      viewRA = ra;
      viewDec = dec;
      pixPerDeg = start.ppd * (t.ppd / start.ppd) ** e;
      anim = k < 1 ? requestAnimationFrame(step) : null;
    };
    anim = requestAnimationFrame(step);
  }

  function targetsOf(entries: app.AtlasIndexEntry[]): FitTarget[] {
    return entries.map((e) => ({
      ra: e.ra,
      dec: e.dec,
      radius: frameExtentDeg(e, sizeQueue.get(e.nasPath)) / 2,
    }));
  }

  /**
   * Fits targets into the canvas area not covered by the object list (left)
   * and, optionally, the frame panel (right), then shifts the centre so they
   * land in the middle of that free area.
   */
  function fitTargets(targets: FitTarget[], opts: { cluster?: boolean; panel?: boolean } = {}) {
    const wide = canvasW >= 760;
    const left = wide && index.length > 0 ? 250 : 0;
    const right = wide && opts.panel ? 290 : 0;
    const freeW = canvasW - left - right;
    const r = fitView(targets, freeW, canvasH, { cluster: opts.cluster, maxPpd: canvasW / 0.5 });
    if (!r) return;
    const dx = left + freeW / 2 - canvasW / 2; // where the target should sit, relative to centre
    const [ra, dec] = fromTangent(r.ra, r.dec, dx / r.ppd, 0);
    goTo({ ra, dec, ppd: r.ppd });
  }

  function fitAll() {
    fitTargets(targetsOf(visibleIndex.length > 0 ? visibleIndex : index), {
      panel: panelEntry !== null,
    });
  }

  // Fit once the index has loaded and the canvas is actually visible.
  let pendingFit = false;
  function requestFit() {
    pendingFit = true;
    tryPendingFit();
  }
  function tryPendingFit() {
    if (!pendingFit || !active || canvasW < 50 || canvasH < 50 || index.length === 0) return;
    pendingFit = false;
    fitAll();
  }
  $effect(() => {
    void active;
    void canvasW;
    void canvasH;
    untrack(tryPendingFit);
  });

  function zoomBy(factor: number, x = canvasW / 2, y = canvasH / 2) {
    stopAnim();
    const old = pixPerDeg;
    const ppd = clampPpd(old * factor);
    const c = zoomAnchored({ ...vp, ppd }, old, x, y);
    pixPerDeg = ppd;
    viewRA = c.ra;
    viewDec = c.dec;
  }

  function panBy(dx: number, dy: number) {
    stopAnim();
    const c = panFrom(viewRA, viewDec, pixPerDeg, dx, dy);
    viewRA = c.ra;
    viewDec = c.dec;
  }

  // ── Selection actions ─────────────────────────────────────────────────────
  /** Prepares a frame for overlay: retry a failed preview, fetch its size first. */
  function prime(e: app.AtlasIndexEntry) {
    previews.retry(e.nasPath);
    if (!sizeQueue.get(e.nasPath)) sizeQueue.request([e.nasPath, ...framesNeedingSize(scene)]);
  }

  function selectOnly(e: app.AtlasIndexEntry) {
    prime(e);
    overlayPaths = [e.nasPath];
    focus = null;
  }

  function toggleOverlay(e: app.AtlasIndexEntry) {
    if (overlayPaths.includes(e.nasPath)) {
      overlayPaths = overlayPaths.filter((p) => p !== e.nasPath);
    } else {
      prime(e);
      overlayPaths = [...overlayPaths, e.nasPath];
    }
    focus = null;
  }

  function bringToFront(path: string) {
    overlayPaths = [...overlayPaths.filter((p) => p !== path), path];
  }

  function removeFromOverlay(path: string) {
    overlayPaths = overlayPaths.filter((p) => p !== path);
  }

  function clearSelection() {
    overlayPaths = [];
    focus = null;
  }

  function selectGroup(g: ObjectGroup) {
    const top = topFrame(g.frames);
    if (top) selectOnly(top);
    fitTargets(targetsOf(g.frames), { cluster: false, panel: true });
  }

  function selectCatalog(obj: app.CatalogObject) {
    const ppd = Math.min(Math.max(pixPerDeg, canvasW / 30), canvasW / 3);
    goTo({ ra: obj.ra, dec: obj.dec, ppd });
    focus = { ra: obj.ra, dec: obj.dec };
  }

  function setFix(path: string, fix: OrientationFix) {
    fixes = { ...fixes, [path]: fix };
  }

  // ── Pointer ───────────────────────────────────────────────────────────────
  let isPanning = $state(false);
  let dragMoved = $state(false);
  let panStart = { x: 0, y: 0, ra: 0, dec: 0 };

  function localXY(e: MouseEvent): [number, number] {
    const rect = canvas.getBoundingClientRect();
    return [e.clientX - rect.left, e.clientY - rect.top];
  }

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const [mx, my] = localXY(e);
    const factor = Math.min(1.5, Math.max(0.66, Math.exp(-e.deltaY * 0.0015)));
    zoomBy(factor, mx, my);
  }

  function onMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;
    stopAnim();
    isPanning = true;
    dragMoved = false;
    panStart = { x: e.clientX, y: e.clientY, ra: viewRA, dec: viewDec };
  }

  function onMouseMove(e: MouseEvent) {
    const [mx, my] = localXY(e);
    if (isPanning) {
      const dx = e.clientX - panStart.x;
      const dy = e.clientY - panStart.y;
      if (!dragMoved && Math.hypot(dx, dy) <= 4) return;
      dragMoved = true;
      const c = panFrom(panStart.ra, panStart.dec, pixPerDeg, dx, dy);
      viewRA = c.ra;
      viewDec = c.dec;
      hoveredEntry = null;
      return;
    }
    hoveredEntry = hitTest(scene, mx, my);
    hoverX = mx;
    hoverY = my;
  }

  function onMouseUp(e: MouseEvent) {
    if (!isPanning) return;
    isPanning = false;
    if (dragMoved) return;
    const [mx, my] = localXY(e);
    const hit = hitTest(scene, mx, my);
    if (!hit) clearSelection();
    else if (e.shiftKey || e.ctrlKey || e.metaKey) toggleOverlay(hit);
    else selectOnly(hit);
  }

  // ── Keyboard ──────────────────────────────────────────────────────────────
  let browser = $state<ReturnType<typeof ObjectBrowser> | null>(null);

  function onKeydown(e: KeyboardEvent) {
    if (!active || isTypingTarget(e) || isModalOpen()) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const step = Math.min(canvasW, canvasH) / 6;
    switch (e.key) {
      case "+":
      case "=":
        zoomBy(1.4);
        break;
      case "-":
      case "_":
        zoomBy(1 / 1.4);
        break;
      case "0":
        fitAll();
        break;
      case "ArrowLeft":
        panBy(step, 0);
        break;
      case "ArrowRight":
        panBy(-step, 0);
        break;
      case "ArrowUp":
        panBy(0, step);
        break;
      case "ArrowDown":
        panBy(0, -step);
        break;
      case "Escape":
        if (helpOpen) helpOpen = false;
        else if (overlayPaths.length > 0 || focus) clearSelection();
        else return;
        break;
      case "/":
        if (!browser) return;
        browser.focusSearch();
        break;
      default:
        return;
    }
    e.preventDefault();
  }

  // ── Loading ───────────────────────────────────────────────────────────────
  let loadSeq = 0;
  async function loadIndex(fit: boolean) {
    const seq = ++loadSeq;
    if (!rootPath) {
      index = [];
      loading = false;
      return;
    }
    try {
      const result = (await GetAtlasIndex(rootPath)) ?? [];
      if (seq !== loadSeq) return;
      const wasEmpty = index.length === 0;
      index = result;
      loadError = "";
      hoveredEntry = null;
      // Keep the selection for frames that still exist.
      const known = new Set(result.map((e) => e.nasPath));
      if (overlayPaths.some((p) => !known.has(p))) {
        overlayPaths = overlayPaths.filter((p) => known.has(p));
      }
      // Fit on first load, and when a scan populates a previously empty atlas.
      if (fit || wasEmpty) requestFit();
    } catch (err) {
      if (seq === loadSeq) loadError = String(err);
    } finally {
      if (seq === loadSeq) loading = false;
    }
  }

  // (Re)load whenever the library root changes — including the first mount.
  $effect(() => {
    void rootPath;
    untrack(() => {
      sizeQueue.reset();
      previews.clear();
      overlayPaths = [];
      filterMode = null;
      focus = null;
      fixes = {};
      loading = true;
      loadError = "";
      loadIndex(true);
    });
  });

  onMount(() => {
    ctx = canvas.getContext("2d");
    dpr = window.devicePixelRatio || 1;

    const ro = new ResizeObserver((entries) => {
      const r = entries[0].contentRect;
      // Hidden tabs report 0×0; keep the last real size.
      if (r.width > 0 && r.height > 0) {
        canvasW = r.width;
        canvasH = r.height;
      }
      dpr = window.devicePixelRatio || 1;
    });
    ro.observe(container);
    if (container.clientWidth > 0 && container.clientHeight > 0) {
      canvasW = container.clientWidth;
      canvasH = container.clientHeight;
    }

    GetCatalog()
      .then((r) => (catalog = r ?? []))
      .catch((err) => toast.error(`Couldn't load the star catalog: ${String(err)}`));

    const unsubUpdated = Events.On("library:updated", () => loadIndex(false));

    return () => {
      ro.disconnect();
      stopAnim();
      if (lazyTimer) clearTimeout(lazyTimer);
      unsubUpdated();
    };
  });

  // ── HUD formatting ────────────────────────────────────────────────────────
  let fovDeg = $derived(canvasW / pixPerDeg);
  let fovText = $derived(
    fovDeg < 1 ? `${(fovDeg * 60).toFixed(1)}′` : `${fovDeg.toFixed(fovDeg < 10 ? 1 : 0)}°`,
  );
</script>

<svelte:window onkeydown={onKeydown} />

<div class="atlas-root" bind:this={container}>
  <canvas
    bind:this={canvas}
    class="atlas-canvas"
    width={Math.round(canvasW * dpr)}
    height={Math.round(canvasH * dpr)}
    style:width="{canvasW}px"
    style:height="{canvasH}px"
    style:cursor={isPanning && dragMoved ? "grabbing" : hoveredEntry ? "pointer" : "grab"}
    aria-label="Sky atlas: your frames plotted on the sky"
    onwheel={onWheel}
    onmousedown={onMouseDown}
    onmousemove={onMouseMove}
    onmouseup={onMouseUp}
    onmouseleave={() => {
      isPanning = false;
      hoveredEntry = null;
    }}
  ></canvas>

  <AtlasStatus
    {rootPath}
    {loading}
    error={loadError}
    indexed={index.length}
    visible={visibleIndex.length}
    onretry={() => {
      loading = true;
      loadIndex(true);
    }}
    onshowall={() => {
      filterMode = "all";
      requestFit();
    }}
    {onscan}
  />

  <!-- HUD -->
  <div class="atlas-hud">
    <span title="{viewRA.toFixed(4)}°">RA {formatRA(viewRA)}</span>
    <span title="{viewDec.toFixed(4)}°">Dec {formatDec(viewDec)}</span>
    <span>FOV {fovText}</span>
    <span class="hud-sep" aria-hidden="true"></span>
    <span>
      {visibleIndex.length} frame{visibleIndex.length === 1 ? "" : "s"}{hiddenCount > 0
        ? ` (${hiddenCount} hidden)`
        : ""}
    </span>
  </div>

  {#if !loading && !loadError && index.length > 0}
    <ObjectBrowser
      bind:this={browser}
      groups={objectGroups}
      {catalog}
      activeName={activeObject}
      onselectgroup={selectGroup}
      onselectcatalog={selectCatalog}
    />
  {/if}

  <!-- Top-right control cluster: frame filter, labels, help -->
  <div class="atlas-controls">
    <div class="seg" role="group" aria-label="Frames to show">
      <button
        class="seg-btn"
        class:active={!showAll}
        aria-pressed={!showAll}
        title="Only processed / exported images"
        onclick={() => (filterMode = "final")}
        >Final images <span class="seg-count">{finalCount}</span></button
      >
      <button
        class="seg-btn"
        class:active={showAll}
        aria-pressed={showAll}
        title="Stacked and processed frames"
        onclick={() => (filterMode = "all")}
        >All frames <span class="seg-count">{index.length}</span></button
      >
    </div>
    <button
      class="ctl-btn"
      class:active={showLabels}
      aria-pressed={showLabels}
      onclick={() => (showLabels = !showLabels)}
      title={showLabels ? "Hide labels" : "Show labels"}>Labels</button
    >
    <HelpPopover bind:open={helpOpen} />
  </div>

  <!-- Zoom controls -->
  <div class="atlas-zoom" role="group" aria-label="Zoom">
    <button class="zoom-btn" title="Zoom in (+)" aria-label="Zoom in" onclick={() => zoomBy(1.4)}
      >+</button
    >
    <button
      class="zoom-btn"
      title="Zoom out (−)"
      aria-label="Zoom out"
      onclick={() => zoomBy(1 / 1.4)}>−</button
    >
    <button class="zoom-btn" title="Fit all frames (0)" aria-label="Fit all frames" onclick={fitAll}
      >⤢</button
    >
  </div>

  {#if sizesBusy > 0 || previewsBusy > 0}
    <div class="atlas-busy" role="status">
      <div class="busy-spinner" aria-hidden="true"></div>
      <span>{previewsBusy > 0 ? "Loading preview…" : `Loading frame outlines… (${sizesBusy})`}</span
      >
    </div>
  {/if}

  {#if hoveredEntry && !isPanning && !helpOpen}
    <AtlasTooltip
      entry={hoveredEntry}
      x={hoverX}
      y={hoverY}
      boundsW={canvasW}
      boundsH={canvasH}
      overlaid={overlayPaths.includes(hoveredEntry.nasPath)}
      selected={panelEntry?.nasPath === hoveredEntry.nasPath && overlay.length === 1}
    />
  {/if}

  {#if panelEntry}
    <AtlasPanel
      entry={panelEntry}
      size={cacheVersion >= 0 ? sizeQueue.get(panelEntry.nasPath) : undefined}
      {overlay}
      previewLoading={cacheVersion >= 0 && previews.isLoading(panelEntry.nasPath)}
      fix={fixes[panelEntry.nasPath] ?? NO_FIX}
      onclose={() => panelEntry && removeFromOverlay(panelEntry.nasPath)}
      onbringtofront={bringToFront}
      onremove={removeFromOverlay}
      onclearall={clearSelection}
      onfixchange={(fix) => panelEntry && setFix(panelEntry.nasPath, fix)}
      onopen={onframeopen}
    />
  {/if}
</div>

<style>
  .atlas-root {
    flex: 1;
    position: relative;
    overflow: hidden;
    background: #050610;
    display: flex;
    align-items: stretch;
  }

  .atlas-canvas {
    display: block;
    position: absolute;
    inset: 0;
    z-index: 0;
  }

  /* HUD */
  .atlas-hud {
    position: absolute;
    top: 10px;
    left: 10px;
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: var(--fs-xs);
    font-variant-numeric: tabular-nums;
    color: var(--text-primary);
    background: color-mix(in srgb, var(--bg-panel) 90%, transparent);
    padding: 4px 10px;
    border-radius: 5px;
    border: 1px solid var(--border);
    backdrop-filter: blur(2px);
    z-index: 20;
    white-space: nowrap;
  }
  .hud-sep {
    width: 1px;
    align-self: stretch;
    background: var(--border-accent);
  }

  /* Top-right control cluster */
  .atlas-controls {
    position: absolute;
    top: 8px;
    right: 10px;
    display: flex;
    align-items: center;
    gap: 8px;
    z-index: 40;
  }
  .seg {
    display: flex;
    background: color-mix(in srgb, var(--bg-panel) 92%, transparent);
    border: 1px solid var(--border);
    border-radius: 5px;
    overflow: hidden;
    backdrop-filter: blur(2px);
  }
  .seg-btn,
  .ctl-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 12px;
    font-size: var(--fs-xs);
    font-weight: 600;
    color: var(--text-secondary);
    background: none;
    border: none;
    cursor: pointer;
  }
  .seg-btn + .seg-btn {
    border-left: 1px solid var(--border);
  }
  .seg-count {
    font-weight: 400;
    font-variant-numeric: tabular-nums;
  }
  .ctl-btn {
    background: color-mix(in srgb, var(--bg-panel) 92%, transparent);
    border: 1px solid var(--border);
    border-radius: 5px;
    backdrop-filter: blur(2px);
  }
  .seg-btn:hover:not(.active),
  .ctl-btn:hover:not(.active) {
    color: var(--text-primary);
    background: var(--bg-row-hover);
  }
  .seg-btn.active,
  .ctl-btn.active {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .ctl-btn.active {
    border-color: var(--accent);
  }

  /* Zoom controls, bottom-right */
  .atlas-zoom {
    position: absolute;
    bottom: 16px;
    right: 10px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    z-index: 20;
  }
  .zoom-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: color-mix(in srgb, var(--bg-panel) 92%, transparent);
    border: 1px solid var(--border);
    border-radius: 5px;
    color: var(--text-primary);
    font-size: var(--fs-lg);
    line-height: 1;
    cursor: pointer;
    backdrop-filter: blur(2px);
  }
  .zoom-btn:hover {
    color: var(--accent);
    border-color: var(--accent);
  }

  /* Background-loading indicator, bottom-centre */
  .atlas-busy {
    position: absolute;
    bottom: 16px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 8px;
    background: color-mix(in srgb, var(--bg-panel) 94%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 5px 12px;
    font-size: var(--fs-xs);
    color: var(--text-primary);
    z-index: 25;
    pointer-events: none;
  }
  .busy-spinner {
    width: 14px;
    height: 14px;
    border: 2px solid color-mix(in srgb, var(--accent) 25%, transparent);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
    flex-shrink: 0;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
