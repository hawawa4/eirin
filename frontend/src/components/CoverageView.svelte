<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { Events } from "@wailsio/runtime";
  import type * as app from "$models/app";
  import { GetCatalog, GetLightCoverage } from "$app";
  import { formatDec, formatRA, plural } from "../lib/utils";
  import { toast } from "../lib/toast.svelte";
  import { isModalOpen, isTypingTarget } from "../lib/keys";
  import type { Project, Theme } from "../lib/types";
  import { SkyCamera } from "../lib/atlas/camera.svelte";
  import { fitView, type FitTarget } from "../lib/atlas/fit";
  import { fromTangent } from "../lib/atlas/projection";
  import { atlasPalette, paletteCssVars, rgba, type RGB } from "../lib/atlas/palette";
  import { drawCoverage } from "../lib/coverage/draw";
  import { clusterRadius, hitTest, isPlaced, type CoverageScene } from "../lib/coverage/scene";
  import {
    formatIntegration,
    objectGroups,
    suggestName,
    summarize,
    type CoverageGroup,
  } from "../lib/coverage/selection";
  import ObjectBrowser from "./atlas/ObjectBrowser.svelte";
  import CoverageTooltip from "./coverage/CoverageTooltip.svelte";
  import CoveragePanel from "./coverage/CoveragePanel.svelte";
  import AddToProjectModal from "./coverage/AddToProjectModal.svelte";
  import CreateProjectModal from "./library/CreateProjectModal.svelte";

  interface Props {
    rootPath: string;
    /** True while this view's tab is visible. Keyboard shortcuts, rendering and reloads pause otherwise. */
    active?: boolean;
    theme?: Theme;
    /** Request a library scan — offered when there are no lights yet. */
    onscan?: () => void;
    /** Opens a project in the Projects view. */
    onopenproject: (project: Project) => void;
    /** Shows these frames, selected, in the Library. */
    onshowinlibrary: (paths: string[]) => void;
  }

  let {
    rootPath,
    active = true,
    theme = "blue",
    onscan,
    onopenproject,
    onshowinlibrary,
  }: Props = $props();

  let palette = $derived(atlasPalette(theme));

  // ── Canvas & camera ───────────────────────────────────────────────────────
  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D | null = null;
  let container: HTMLElement;
  let dpr = $state(1);
  const cam = new SkyCamera();

  // ── Data ──────────────────────────────────────────────────────────────────
  let coverage = $state.raw<app.LightCoverage>({ clusters: [], unplaced: [], scopes: [] });
  let catalog = $state.raw<app.CatalogObject[]>([]);
  let loading = $state(true);
  let loadError = $state("");
  /** A library change arrived while the tab was hidden. */
  let stale = false;

  let all = $derived([...coverage.clusters, ...coverage.unplaced]);
  let byId = $derived(new Map(all.map((c) => [c.id, c])));
  let totalLights = $derived(coverage.scopes.reduce((n, s) => n + s.frames, 0));

  // ── Scopes ────────────────────────────────────────────────────────────────
  const hiddenScopes = new SvelteSet<string>();
  let scopeLabels = $derived(new Map(coverage.scopes.map((s) => [s.scope, s.label])));
  let scopeColors = $derived(
    new Map(
      coverage.scopes.map((s, i) => [s.scope, palette.scopes[i % palette.scopes.length]] as const),
    ),
  );
  const scopeLabel = (scope: string) => scopeLabels.get(scope) ?? scope;
  const scopeColor = (scope: string): RGB => scopeColors.get(scope) ?? palette.frame;
  const scopeCss = (scope: string) => rgba(scopeColor(scope));

  function toggleScope(scope: string) {
    if (!hiddenScopes.delete(scope)) hiddenScopes.add(scope);
  }

  let visible = $derived(coverage.clusters.filter((c) => !hiddenScopes.has(c.scope)));
  let groups = $derived(objectGroups(all.filter((c) => !hiddenScopes.has(c.scope))));

  let showLabels = $state(true);

  // ── Selection ─────────────────────────────────────────────────────────────
  // Cluster ids in selection order; ids (not clusters) so a reload keeps them.
  let selectedIds = $state.raw<string[]>([]);
  let selectedSet = $derived(new Set(selectedIds));
  let selected = $derived(
    selectedIds.map((id) => byId.get(id)).filter((c): c is app.CoverageCluster => !!c),
  );
  let activeObject = $state<string | null>(null);

  let hovered = $state.raw<app.CoverageCluster | null>(null);
  let hoverX = $state(0);
  let hoverY = $state(0);
  let focus = $state.raw<{ ra: number; dec: number } | null>(null);

  function selectOnly(c: app.CoverageCluster) {
    selectedIds = [c.id];
    activeObject = null;
    focus = null;
  }

  function toggle(c: app.CoverageCluster) {
    selectedIds = selectedSet.has(c.id)
      ? selectedIds.filter((id) => id !== c.id)
      : [...selectedIds, c.id];
    focus = null;
  }

  function add(c: app.CoverageCluster) {
    if (!selectedSet.has(c.id)) selectedIds = [...selectedIds, c.id];
  }

  function remove(c: app.CoverageCluster) {
    selectedIds = selectedIds.filter((id) => id !== c.id);
  }

  function clearSelection() {
    selectedIds = [];
    activeObject = null;
    focus = null;
  }

  function selectGroup(g: CoverageGroup, additive: boolean) {
    const ids = additive
      ? [...selectedIds, ...g.clusterIds.filter((id) => !selectedSet.has(id))]
      : g.clusterIds;
    selectedIds = ids;
    activeObject = g.name;
    focus = null;
    const targets = g.clusterIds.map((id) => byId.get(id)).filter((c) => c && isPlaced(c));
    fitTargets(targetsOf(targets as app.CoverageCluster[]), true);
  }

  function selectCatalog(obj: app.CatalogObject) {
    const ppd = Math.min(Math.max(cam.ppd, cam.w / 30), cam.w / 3);
    cam.goTo({ ra: obj.ra, dec: obj.dec, ppd });
    focus = { ra: obj.ra, dec: obj.dec };
  }

  function focusCluster(c: app.CoverageCluster) {
    if (isPlaced(c)) fitTargets(targetsOf([c]), true);
  }

  // ── Scene & rendering ─────────────────────────────────────────────────────
  let scene = $derived<CoverageScene>({
    vp: cam.vp,
    clusters: visible,
    selected: selectedSet,
    hoveredId: hovered?.id ?? null,
    catalog,
    showLabels,
    palette,
    focus,
    scopeColor,
  });

  let redrawPending = false;
  function scheduleRedraw() {
    if (redrawPending) return;
    redrawPending = true;
    requestAnimationFrame(() => {
      redrawPending = false;
      if (!ctx) return;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      drawCoverage(ctx, scene);
    });
  }

  $effect(() => {
    void scene;
    void dpr;
    if (active) scheduleRedraw();
  });

  // ── Fitting ───────────────────────────────────────────────────────────────
  function targetsOf(clusters: app.CoverageCluster[]): FitTarget[] {
    return clusters.map((c) => ({ ra: c.ra, dec: c.dec, radius: clusterRadius(c) }));
  }

  /** Fits targets into the canvas area left free by the object list and the panel. */
  function fitTargets(targets: FitTarget[], panel: boolean) {
    if (targets.length === 0) return;
    const wide = cam.w >= 760;
    const left = wide ? 250 : 0;
    const right = wide && panel ? 310 : 0;
    const freeW = cam.w - left - right;
    const r = fitView(targets, freeW, cam.h, { maxPpd: cam.w / 0.5 });
    if (!r) return;
    const [ra, dec] = fromTangent(r.ra, r.dec, (left + freeW / 2 - cam.w / 2) / r.ppd, 0);
    cam.goTo({ ra, dec, ppd: r.ppd });
  }

  function fitAll() {
    fitTargets(targetsOf(visible.length > 0 ? visible : coverage.clusters), selected.length > 0);
  }

  let pendingFit = false;
  function tryPendingFit() {
    if (!pendingFit || !active || cam.w < 50 || cam.h < 50 || coverage.clusters.length === 0) {
      return;
    }
    pendingFit = false;
    fitAll();
  }
  $effect(() => {
    void active;
    void cam.w;
    void cam.h;
    untrack(tryPendingFit);
  });

  // ── Pointer ───────────────────────────────────────────────────────────────
  function localXY(e: MouseEvent): [number, number] {
    const rect = canvas.getBoundingClientRect();
    return [e.clientX - rect.left, e.clientY - rect.top];
  }

  function onMouseMove(e: MouseEvent) {
    if (cam.dragTo(e)) {
      hovered = null;
      return;
    }
    if (cam.isPanning) return;
    const [mx, my] = localXY(e);
    hovered = hitTest(scene, mx, my);
    hoverX = mx;
    hoverY = my;
  }

  function onMouseUp(e: MouseEvent) {
    if (cam.endDrag() !== "click") return;
    const hit = hitTest(scene, ...localXY(e));
    if (!hit) clearSelection();
    else if (e.shiftKey || e.ctrlKey || e.metaKey) toggle(hit);
    else selectOnly(hit);
  }

  // ── Keyboard ──────────────────────────────────────────────────────────────
  let browser = $state<ReturnType<typeof ObjectBrowser> | null>(null);

  function onKeydown(e: KeyboardEvent) {
    if (!active || isTypingTarget(e) || isModalOpen()) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (cam.navKey(e.key)) {
      e.preventDefault();
      return;
    }
    switch (e.key) {
      case "0":
        fitAll();
        break;
      case "Escape":
        if (selectedIds.length === 0 && !focus) return;
        clearSelection();
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

  // ── Actions ───────────────────────────────────────────────────────────────
  let createPaths = $state.raw<string[] | null>(null);
  let addPaths = $state.raw<string[] | null>(null);
  let summary = $derived(summarize(selected));

  // ── Loading ───────────────────────────────────────────────────────────────
  let loadSeq = 0;
  async function load(fit: boolean) {
    const seq = ++loadSeq;
    stale = false;
    try {
      const result = await GetLightCoverage(rootPath);
      if (seq !== loadSeq) return;
      const wasEmpty = coverage.clusters.length === 0;
      coverage = result;
      loadError = "";
      hovered = null;
      const known = new Set([...result.clusters, ...result.unplaced].map((c) => c.id));
      if (selectedIds.some((id) => !known.has(id))) {
        selectedIds = selectedIds.filter((id) => known.has(id));
      }
      if (fit || wasEmpty) {
        pendingFit = true;
        tryPendingFit();
      }
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
      selectedIds = [];
      hiddenScopes.clear();
      focus = null;
      loading = true;
      loadError = "";
      void load(true);
    });
  });

  // Catch up on library changes that happened while hidden.
  $effect(() => {
    if (active && stale) untrack(() => void load(false));
  });

  onMount(() => {
    ctx = canvas.getContext("2d");
    dpr = window.devicePixelRatio || 1;

    const ro = new ResizeObserver((entries) => {
      const r = entries[0].contentRect;
      // Hidden tabs report 0×0; keep the last real size.
      if (r.width > 0 && r.height > 0) {
        cam.w = r.width;
        cam.h = r.height;
      }
      dpr = window.devicePixelRatio || 1;
    });
    ro.observe(container);
    if (container.clientWidth > 0 && container.clientHeight > 0) {
      cam.w = container.clientWidth;
      cam.h = container.clientHeight;
    }

    GetCatalog()
      .then((r) => (catalog = r ?? []))
      .catch((err) => toast.error(`Couldn't load the star catalog: ${String(err)}`));

    const unsubUpdated = Events.On("library:updated", () => {
      if (active) void load(false);
      else stale = true;
    });

    return () => {
      ro.disconnect();
      cam.stopAnim();
      unsubUpdated();
    };
  });

  // ── HUD ───────────────────────────────────────────────────────────────────
  let fovText = $derived(
    cam.fovDeg < 1
      ? `${(cam.fovDeg * 60).toFixed(1)}′`
      : `${cam.fovDeg.toFixed(cam.fovDeg < 10 ? 1 : 0)}°`,
  );
</script>

<svelte:window onkeydown={onKeydown} />

<div class="cov-root" bind:this={container} style={paletteCssVars(palette)}>
  <canvas
    bind:this={canvas}
    class="cov-canvas"
    width={Math.round(cam.w * dpr)}
    height={Math.round(cam.h * dpr)}
    style:width="{cam.w}px"
    style:height="{cam.h}px"
    style:cursor={cam.isPanning && cam.dragMoved ? "grabbing" : hovered ? "pointer" : "grab"}
    aria-label="Coverage: the sky framed by your light frames, by telescope"
    onwheel={(e) => cam.wheel(e, ...localXY(e))}
    onmousedown={(e) => cam.beginDrag(e)}
    onmousemove={onMouseMove}
    onmouseup={onMouseUp}
    onmouseleave={() => {
      cam.endDrag();
      hovered = null;
    }}
  ></canvas>

  {#if loading || loadError || totalLights === 0}
    <div class="cov-status">
      {#if loading}
        <p>Grouping your light frames…</p>
      {:else if loadError}
        <p class="err">Couldn't load coverage: {loadError}</p>
        <button
          class="btn-primary"
          onclick={() => {
            loading = true;
            void load(true);
          }}>Retry</button
        >
      {:else}
        <p>No light frames in the library yet.</p>
        {#if onscan}<button class="btn-primary" onclick={onscan}>Scan library</button>{/if}
      {/if}
    </div>
  {/if}

  <div class="cov-hud">
    <span title="{cam.ra.toFixed(4)}°">RA {formatRA(cam.ra)}</span>
    <span title="{cam.dec.toFixed(4)}°">Dec {formatDec(cam.dec)}</span>
    <span>FOV {fovText}</span>
    <span class="hud-sep" aria-hidden="true"></span>
    <span>{plural(visible.length, "framing")}</span>
    {#if selected.length > 0}
      <span class="hud-sel">
        {plural(summary.frames, "light")} · {formatIntegration(summary.expTotal)} selected
      </span>
    {/if}
  </div>

  {#if !loading && !loadError && all.length > 0}
    <ObjectBrowser
      bind:this={browser}
      {groups}
      {catalog}
      activeName={activeObject}
      onselectgroup={selectGroup}
      onselectcatalog={selectCatalog}
    />
  {/if}

  <!-- Top-right: scope legend (click to show/hide) and labels -->
  <div class="cov-controls">
    {#each coverage.scopes as s (s.scope)}
      {@const shown = !hiddenScopes.has(s.scope)}
      <button
        class="scope-chip"
        class:off={!shown}
        aria-pressed={shown}
        onclick={() => toggleScope(s.scope)}
        title="{shown ? 'Hide' : 'Show'} {s.label} ({plural(s.frames, 'light')})"
      >
        <span class="scope-swatch" style:border-color={scopeCss(s.scope)}></span>
        {s.label}
        <span class="scope-count">{s.frames}</span>
      </button>
    {/each}
    <button
      class="ctl-btn"
      class:active={showLabels}
      aria-pressed={showLabels}
      onclick={() => (showLabels = !showLabels)}
      title={showLabels ? "Hide labels" : "Show labels"}>Labels</button
    >
  </div>

  <div class="cov-zoom" role="group" aria-label="Zoom">
    <button
      class="zoom-btn"
      title="Zoom in (+)"
      aria-label="Zoom in"
      onclick={() => cam.zoomBy(1.4)}>+</button
    >
    <button
      class="zoom-btn"
      title="Zoom out (−)"
      aria-label="Zoom out"
      onclick={() => cam.zoomBy(1 / 1.4)}>−</button
    >
    <button class="zoom-btn" title="Fit all (0)" aria-label="Fit all framings" onclick={fitAll}
      >⤢</button
    >
  </div>

  {#if selected.length === 0 && !loading && all.length > 0}
    <div class="cov-hint">
      Click a framing to select it · Shift/Ctrl+click to add more · dashed = estimated position
    </div>
  {/if}

  {#if hovered && !cam.isPanning}
    <CoverageTooltip
      cluster={hovered}
      scopeLabel={scopeLabel(hovered.scope)}
      x={hoverX}
      y={hoverY}
      boundsW={cam.w}
      boundsH={cam.h}
      selected={selectedSet.has(hovered.id)}
    />
  {/if}

  {#if selected.length > 0}
    <CoveragePanel
      {selected}
      {all}
      {scopeLabel}
      {scopeCss}
      onfocus={focusCluster}
      onadd={add}
      onremove={remove}
      onclear={clearSelection}
      oncreateproject={() => (createPaths = summary.paths)}
      onaddtoproject={() => (addPaths = summary.paths)}
      onshowinlibrary={() => onshowinlibrary(summary.paths)}
    />
  {/if}
</div>

{#if createPaths}
  <CreateProjectModal
    frames={createPaths.map((nasPath) => ({ nasPath, frameType: "light" }))}
    suggestedName={suggestName(summary, scopeLabel)}
    onclose={() => (createPaths = null)}
    oncreated={onopenproject}
  />
{/if}

{#if addPaths}
  <AddToProjectModal paths={addPaths} onclose={() => (addPaths = null)} {onopenproject} />
{/if}

<style>
  .cov-root {
    flex: 1;
    position: relative;
    overflow: hidden;
    background: var(--atlas-bg);
    display: flex;
    align-items: stretch;
  }
  .cov-canvas {
    display: block;
    position: absolute;
    inset: 0;
    z-index: 0;
  }

  .cov-status {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    padding: 16px 22px;
    background: color-mix(in srgb, var(--bg-panel) 94%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 7px;
    z-index: 35;
    font-size: var(--fs-sm);
    color: var(--text-primary);
  }
  .cov-status p {
    margin: 0;
  }
  .cov-status .err {
    color: var(--danger);
  }

  .cov-hud {
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
  .hud-sel {
    color: var(--atlas-selected);
    font-weight: 600;
  }

  .cov-controls {
    position: absolute;
    top: 8px;
    right: 10px;
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    align-items: center;
    gap: 6px;
    max-width: 55%;
    z-index: 40;
  }
  .scope-chip,
  .ctl-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    font-size: var(--fs-xs);
    font-weight: 600;
    color: var(--text-primary);
    background: color-mix(in srgb, var(--bg-panel) 92%, transparent);
    border: 1px solid var(--border);
    border-radius: 5px;
    backdrop-filter: blur(2px);
    cursor: pointer;
    white-space: nowrap;
  }
  .scope-chip:hover,
  .ctl-btn:hover:not(.active) {
    background: var(--bg-row-hover);
  }
  .scope-chip.off {
    color: var(--text-dim);
  }
  .scope-chip.off .scope-swatch {
    opacity: 0.35;
  }
  .scope-swatch {
    width: 11px;
    height: 11px;
    border: 2px solid;
    border-radius: 2px;
  }
  .scope-count {
    font-weight: 400;
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
  }
  .ctl-btn {
    color: var(--text-secondary);
  }
  .ctl-btn.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }

  .cov-zoom {
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

  .cov-hint {
    position: absolute;
    bottom: 16px;
    left: 50%;
    transform: translateX(-50%);
    background: color-mix(in srgb, var(--bg-panel) 90%, transparent);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 5px 12px;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    pointer-events: none;
    z-index: 20;
    white-space: nowrap;
  }
</style>
