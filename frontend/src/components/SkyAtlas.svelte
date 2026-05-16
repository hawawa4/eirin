<script lang="ts">
  import { onMount } from "svelte";
  import type { app } from "../../wailsjs/go/models";
  import { GetAtlasIndex, GetAtlasFrameSize, GetCatalog, GeneratePreview } from "../../wailsjs/go/app/App.js";

  interface Props {
    rootPath: string;
    onframeopen?: (nasPath: string) => void;
  }

  let { rootPath, onframeopen }: Props = $props();

  // ── State ─────────────────────────────────────────────────────────────────
  let canvas: HTMLCanvasElement;
  let container: HTMLElement;
  let canvasW = $state(800);
  let canvasH = $state(600);

  let viewRA  = $state(180);
  let viewDec = $state(0);
  let pixPerDeg = $state(12);

  let index   = $state<app.AtlasIndexEntry[]>([]);
  let catalog = $state<app.CatalogObject[]>([]);
  let loading = $state(true);
  let loadError = $state("");

  // Frame type filter
  let showStacked = $state(false);
  let visibleIndex = $derived(
    showStacked ? index : index.filter((e) => e.frameType === "processed"),
  );

  // Size cache: nasPath → pixel dimensions (plain Map; sizesVersion drives redraws)
  const sizes = new Map<string, { width: number; height: number }>();
  let sizesVersion = $state(0);
  const fetchingPaths = new Set<string>();
  let fetchingCount = $state(0); // reactive counter for in-flight GetAtlasFrameSize calls

  // Pan state
  let isPanning = $state(false);
  let panStartX = 0;
  let panStartY = 0;
  let panStartRA  = 0;
  let panStartDec = 0;

  // Hover / selection
  let hoveredEntry  = $state<app.AtlasIndexEntry | null>(null);
  let hoverX = $state(0);
  let hoverY = $state(0);

  // Multiple selected frames, ordered by click (last = topmost z-order)
  let selectedEntries = $state<app.AtlasIndexEntry[]>([]);

  // Most-recently selected for the side panel
  let panelEntry = $derived(
    selectedEntries.length > 0 ? selectedEntries[selectedEntries.length - 1] : null,
  );

  // Preview images per frame (Map; previewVersion drives redraws)
  const previewImgs = new Map<string, HTMLImageElement>();
  const loadingPaths = new Set<string>();
  let previewVersion = $state(0);

  // Per-frame rotation overrides: -90 | 0 | 90 degrees added on top of stored rotation
  const rotationOverrides = new Map<string, number>();
  let rotOverVersion = $state(0);

  // True when any preview image or frame size is currently being fetched
  let anyLoading = $derived(
    (previewVersion >= 0 && loadingPaths.size > 0) || fetchingCount > 0,
  );

  let lazyTimer: ReturnType<typeof setTimeout> | null = null;
  const LAZY_PPD = 8;

  // ── Preview loading effect ────────────────────────────────────────────────

  $effect(() => {
    // Trigger whenever selectedEntries changes; load images for newly selected frames
    const paths = selectedEntries.map((e) => e.nasPath);

    // Prune images for deselected frames
    for (const key of [...previewImgs.keys()]) {
      if (!paths.includes(key)) {
        previewImgs.delete(key);
        loadingPaths.delete(key);
        previewVersion++;
      }
    }

    // Start loading for frames not yet cached
    for (const entry of selectedEntries) {
      const path = entry.nasPath;
      if (previewImgs.has(path) || loadingPaths.has(path)) continue;
      loadingPaths.add(path);
      GeneratePreview(path, entry.frameType === 'processed' ? 0 : 2)
        .then((url) => {
          const img = new Image();
          img.onload = () => {
            previewImgs.set(path, img);
            loadingPaths.delete(path);
            previewVersion++;
          };
          img.onerror = () => { loadingPaths.delete(path); previewVersion++; };
          img.src = url;
        })
        .catch(() => { loadingPaths.delete(path); previewVersion++; });
    }
  });

  // ── Projection ────────────────────────────────────────────────────────────

  function project(ra: number, dec: number): [number, number] | null {
    const ra0  = viewRA  * Math.PI / 180;
    const dec0 = viewDec * Math.PI / 180;
    const raR  = ra  * Math.PI / 180;
    const decR = dec * Math.PI / 180;

    const dRA   = raR - ra0;
    const denom = Math.sin(dec0) * Math.sin(decR) + Math.cos(dec0) * Math.cos(decR) * Math.cos(dRA);
    if (denom <= 0.001) return null;

    const xi  = Math.cos(decR) * Math.sin(dRA) / denom;
    const eta = (Math.cos(dec0) * Math.sin(decR) - Math.sin(dec0) * Math.cos(decR) * Math.cos(dRA)) / denom;

    return [
      canvasW / 2 - (xi  * 180 / Math.PI) * pixPerDeg,
      canvasH / 2 - (eta * 180 / Math.PI) * pixPerDeg,
    ];
  }

  function unproject(x: number, y: number): [number, number] {
    const xi  = (-(x - canvasW / 2) / pixPerDeg) * Math.PI / 180;
    const eta = (-(y - canvasH / 2) / pixPerDeg) * Math.PI / 180;
    const dec0 = viewDec * Math.PI / 180;
    const ra0  = viewRA  * Math.PI / 180;
    const rho  = Math.sqrt(xi * xi + eta * eta);
    if (rho < 1e-10) return [viewRA, viewDec];
    const c   = Math.atan(rho);
    const dec = Math.asin(Math.cos(c) * Math.sin(dec0) + (eta * Math.sin(c) * Math.cos(dec0)) / rho);
    const ra  = ra0 + Math.atan2(xi * Math.sin(c), rho * Math.cos(dec0) * Math.cos(c) - eta * Math.sin(dec0) * Math.sin(c));
    return [((ra * 180 / Math.PI) + 360) % 360, dec * 180 / Math.PI];
  }

  // ── Footprint helpers ─────────────────────────────────────────────────────

  function footprintCorners(
    entry: app.AtlasIndexEntry,
    sz: { width: number; height: number },
  ): ([number, number] | null)[] {
    const scaleDeg = entry.pixelScale / 3600;
    const hw = (sz.width  / 2) * scaleDeg;
    const hh = (sz.height / 2) * scaleDeg;
    const θ  = entry.rotation * Math.PI / 180;
    const cosDec = Math.cos(entry.dec * Math.PI / 180);
    return (
      [[-hw, -hh], [hw, -hh], [hw, hh], [-hw, hh]] as [number, number][]
    ).map(([dx, dy]) => {
      const rx = dx * Math.cos(θ) - dy * Math.sin(θ);
      const ry = dx * Math.sin(θ) + dy * Math.cos(θ);
      return project(entry.ra + rx / cosDec, entry.dec + ry);
    });
  }

  function isOnScreen(corners: ([number, number] | null)[], margin = 80): boolean {
    return corners.some(
      (c) => c !== null && c[0] >= -margin && c[0] <= canvasW + margin && c[1] >= -margin && c[1] <= canvasH + margin,
    );
  }

  function pointInPolygon(px: number, py: number, pts: [number, number][]): boolean {
    let inside = false;
    for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
      const [xi, yi] = pts[i];
      const [xj, yj] = pts[j];
      if (yi > py !== yj > py && px < ((xj - xi) * (py - yi)) / (yj - yi) + xi) inside = !inside;
    }
    return inside;
  }

  // ── Draw ──────────────────────────────────────────────────────────────────

  function redraw() {
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.clearRect(0, 0, canvasW, canvasH);
    ctx.fillStyle = "#050610";
    ctx.fillRect(0, 0, canvasW, canvasH);
    drawGrid(ctx);
    drawCatalog(ctx);
    drawFrames(ctx);
    drawCompass(ctx);
  }

  function labelBg(ctx: CanvasRenderingContext2D, text: string, x: number, y: number) {
    const tw = ctx.measureText(text).width;
    const saved = ctx.fillStyle;
    ctx.fillStyle = "rgba(5,6,16,0.78)";
    ctx.fillRect(x - 2, y - 11, tw + 4, 14);
    ctx.fillStyle = saved;
    ctx.fillText(text, x, y);
  }

  function drawGrid(ctx: CanvasRenderingContext2D) {
    ctx.save();
    ctx.strokeStyle = "rgba(80,100,140,0.50)";
    ctx.lineWidth = 1.0;
    ctx.setLineDash([4, 6]);
    ctx.font = "11px monospace";

    const decStep = pixPerDeg >= 30 ? 5 : 10;
    const raStep  = pixPerDeg >= 50 ? 5 : 15;

    for (let dec = -90; dec <= 90; dec += decStep) {
      const pts: [number, number][] = [];
      for (let ra = 0; ra <= 360; ra += 2) {
        const p = project(ra, dec);
        if (p) pts.push(p);
      }
      if (pts.length < 2) continue;
      ctx.beginPath();
      ctx.moveTo(pts[0][0], pts[0][1]);
      for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i][0], pts[i][1]);
      ctx.stroke();
    }

    for (let ra = 0; ra < 360; ra += raStep) {
      const pts: [number, number][] = [];
      for (let dec = -85; dec <= 85; dec += 2) {
        const p = project(ra, dec);
        if (p) pts.push(p);
      }
      if (pts.length < 2) continue;
      ctx.beginPath();
      ctx.moveTo(pts[0][0], pts[0][1]);
      for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i][0], pts[i][1]);
      ctx.stroke();
    }

    ctx.setLineDash([]);

    // RA labels (blue-ish) at current Dec centre
    ctx.fillStyle = "rgba(155,180,230,0.95)";
    for (let ra = 0; ra < 360; ra += raStep) {
      const lp = project(ra, viewDec);
      if (lp && lp[0] >= 20 && lp[0] <= canvasW - 20 && lp[1] >= 14 && lp[1] <= canvasH - 4) {
        labelBg(ctx, `${ra}°`, lp[0] + 2, lp[1] - 2);
      }
    }

    // Dec labels (teal-ish) at current RA centre
    ctx.fillStyle = "rgba(110,215,190,0.95)";
    for (let dec = -80; dec <= 80; dec += decStep) {
      const lp = project(viewRA, dec);
      if (lp && lp[1] >= 14 && lp[1] <= canvasH - 4 && lp[0] >= 4 && lp[0] <= canvasW - 4) {
        labelBg(ctx, (dec >= 0 ? "+" : "") + dec + "°", lp[0] + 4, lp[1] - 2);
      }
    }

    ctx.restore();
  }

  function drawCatalog(ctx: CanvasRenderingContext2D) {
    ctx.save();
    for (const obj of catalog) {
      const p = project(obj.ra, obj.dec);
      if (!p) continue;
      const [x, y] = p;
      if (x < -20 || x > canvasW + 20 || y < -20 || y > canvasH + 20) continue;

      if (obj.type === "star") {
        const r = Math.max(0.8, Math.min(3.5, (4 - obj.mag) * 0.6));
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.fillStyle = "rgba(220,230,255,0.85)";
        ctx.fill();
        if (obj.mag < 2.5 || pixPerDeg > 60) {
          ctx.fillStyle = "rgba(180,200,255,0.7)";
          ctx.font = "9px monospace";
          ctx.fillText(obj.name, x + r + 2, y + 3);
        }
      } else {
        const size = pixPerDeg > 20 ? 5 : 3;
        ctx.strokeStyle = "rgba(255,210,80,0.6)";
        ctx.lineWidth = 0.8;
        ctx.beginPath();
        ctx.moveTo(x - size, y); ctx.lineTo(x + size, y);
        ctx.moveTo(x, y - size); ctx.lineTo(x, y + size);
        ctx.stroke();
        if (pixPerDeg > 8) {
          ctx.fillStyle = "rgba(255,210,80,0.7)";
          ctx.font = "9px monospace";
          ctx.fillText(obj.name, x + size + 2, y + 3);
        }
      }
    }
    ctx.restore();
  }

  function drawFrames(ctx: CanvasRenderingContext2D) {
    ctx.save();

    // Draw non-selected frames first (bottom layer)
    for (const entry of visibleIndex) {
      if (selectedEntries.includes(entry)) continue;
      drawSingleFrame(ctx, entry, false, false);
    }

    // Draw selected frames in order (first = bottom, last = top)
    for (const entry of selectedEntries) {
      if (!visibleIndex.includes(entry)) continue;
      drawSingleFrame(ctx, entry, true, false);
    }

    // Draw hovered non-selected on top of everything except selected
    if (hoveredEntry && !selectedEntries.includes(hoveredEntry)) {
      drawSingleFrame(ctx, hoveredEntry, false, true);
    }

    ctx.restore();
  }

  function drawSingleFrame(
    ctx: CanvasRenderingContext2D,
    entry: app.AtlasIndexEntry,
    isSel: boolean,
    isHov: boolean,
  ) {
    const cp = project(entry.ra, entry.dec);
    if (!cp) return;
    const [cx, cy] = cp;

    const sz    = sizes.get(entry.nasPath);
    const img   = previewImgs.get(entry.nasPath);
    const isLoading = loadingPaths.has(entry.nasPath);

    if (sz && pixPerDeg >= LAZY_PPD) {
      const corners = footprintCorners(entry, sz);
      // For frames with loaded images, use a very large margin to avoid unloading when zoomed
      const margin = isSel && img ? 4000 : 80;
      if (!isOnScreen(corners, margin)) return;
      const valid = corners.filter((c): c is [number, number] => c !== null);
      if (valid.length < 3) return;

      if (isSel && img) {
        // Draw actual image aligned to footprint, with optional manual rotation offset
        const rotDeg = entry.rotation + (rotationOverrides.get(entry.nasPath) ?? 0);
        const wPx = sz.width  * entry.pixelScale / 3600 * pixPerDeg;
        const hPx = sz.height * entry.pixelScale / 3600 * pixPerDeg;
        ctx.save();
        ctx.translate(cx, cy);
        ctx.rotate(-rotDeg * Math.PI / 180);
        ctx.drawImage(img, -wPx / 2, -hPx / 2, wPx, hPx);
        ctx.restore();
      } else if (isSel && isLoading) {
        ctx.beginPath();
        ctx.moveTo(valid[0][0], valid[0][1]);
        for (let i = 1; i < valid.length; i++) ctx.lineTo(valid[i][0], valid[i][1]);
        ctx.closePath();
        ctx.fillStyle   = "rgba(255,190,70,0.12)";
        ctx.strokeStyle = "rgba(255,210,80,0.6)";
        ctx.lineWidth = 1.5;
        ctx.setLineDash([6, 4]);
        ctx.fill();
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.fillStyle = "rgba(255,220,100,0.8)";
        ctx.font = "10px monospace";
        ctx.textAlign = "center";
        ctx.fillText("loading…", cx, cy + 4);
        ctx.textAlign = "left";
      } else {
        ctx.beginPath();
        ctx.moveTo(valid[0][0], valid[0][1]);
        for (let i = 1; i < valid.length; i++) ctx.lineTo(valid[i][0], valid[i][1]);
        ctx.closePath();
        ctx.fillStyle   = isHov ? "rgba(100,190,255,0.22)" : isSel ? "rgba(255,190,70,0.10)" : "rgba(80,140,220,0.10)";
        ctx.strokeStyle = isHov ? "rgba(120,210,255,0.95)" : isSel ? "rgba(255,210,80,0.7)" : "rgba(100,160,255,0.55)";
        ctx.lineWidth   = isHov ? 2 : isSel ? 1.8 : 1.2;
        ctx.fill();
        ctx.stroke();
        ctx.fillStyle = isHov ? "rgba(190,230,255,1)" : isSel ? "rgba(255,220,100,0.9)" : "rgba(160,205,255,0.85)";
        ctx.font      = isHov ? "bold 11px monospace" : "10px monospace";
        ctx.textAlign = "center";
        ctx.fillText(entry.object || entry.name, cx, cy + 4);
        ctx.textAlign = "left";
      }
    } else {
      // Not zoomed enough — draw dot
      const r = isSel ? 5 : isHov ? 4 : 3;
      ctx.beginPath();
      ctx.arc(cx, cy, r, 0, Math.PI * 2);
      ctx.fillStyle = isSel ? "rgba(255,210,80,0.95)" : isHov ? "rgba(120,210,255,0.9)" : "rgba(100,165,255,0.65)";
      ctx.fill();
      if (pixPerDeg >= 4) {
        ctx.fillStyle = isSel ? "rgba(255,220,100,1)" : "rgba(165,205,255,0.75)";
        ctx.font = "9px monospace";
        ctx.fillText(entry.object || entry.name, cx + r + 2, cy + 3);
      }
    }
  }

  function drawCompass(ctx: CanvasRenderingContext2D) {
    ctx.save();
    ctx.font = "10px monospace";
    ctx.fillStyle = "rgba(120,150,200,0.6)";
    ctx.fillText("N↑", canvasW - 32, 16);
    ctx.fillText("E→", 8, canvasH / 2);
    ctx.restore();
  }

  // ── Reactive redraw ───────────────────────────────────────────────────────

  $effect(() => {
    void viewRA; void viewDec; void pixPerDeg; void index; void catalog;
    void hoveredEntry; void selectedEntries; void canvasW; void canvasH; void sizesVersion;
    void previewVersion; void showStacked; void rotOverVersion;
    requestAnimationFrame(redraw);
  });

  // ── Lazy size loading ─────────────────────────────────────────────────────

  function scheduleLazyLoad() {
    if (lazyTimer) clearTimeout(lazyTimer);
    lazyTimer = setTimeout(checkLazyLoad, 300);
  }

  function checkLazyLoad() {
    if (pixPerDeg < LAZY_PPD) return;
    for (const entry of visibleIndex) {
      if (sizes.has(entry.nasPath) || fetchingPaths.has(entry.nasPath)) continue;
      const pt = project(entry.ra, entry.dec);
      if (!pt) continue;
      const [x, y] = pt;
      if (x < -200 || x > canvasW + 200 || y < -200 || y > canvasH + 200) continue;
      fetchingPaths.add(entry.nasPath);
      fetchingCount++;
      GetAtlasFrameSize(entry.nasPath)
        .then((sz) => {
          fetchingPaths.delete(entry.nasPath);
          fetchingCount--;
          if (sz.width > 0 && sz.height > 0) {
            sizes.set(entry.nasPath, { width: sz.width, height: sz.height });
            sizesVersion++;
          }
        })
        .catch(() => { fetchingPaths.delete(entry.nasPath); fetchingCount--; });
    }
  }

  // ── Interaction ───────────────────────────────────────────────────────────

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.18 : 0.847;
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    const [skyRA, skyDec] = unproject(mx, my);

    pixPerDeg = Math.max(0.3, Math.min(8000, pixPerDeg * factor));

    const [nx, ny] = project(skyRA, skyDec) ?? [canvasW / 2, canvasH / 2];
    viewRA  += (mx - nx) / pixPerDeg / Math.cos(viewDec * Math.PI / 180);
    viewDec += (my - ny) / pixPerDeg;
    scheduleLazyLoad();
  }

  function onMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;
    isPanning = true;
    panStartX   = e.clientX;
    panStartY   = e.clientY;
    panStartRA  = viewRA;
    panStartDec = viewDec;
  }

  function onMouseMove(e: MouseEvent) {
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;

    if (isPanning) {
      const dx = e.clientX - panStartX;
      const dy = e.clientY - panStartY;
      viewRA  = panStartRA  + dx / pixPerDeg / Math.cos(viewDec * Math.PI / 180);
      viewDec = Math.max(-89, Math.min(89, panStartDec + dy / pixPerDeg));
      hoveredEntry = null;
      scheduleLazyLoad();
      return;
    }

    let found: app.AtlasIndexEntry | null = null;
    for (const entry of visibleIndex) {
      const sz = sizes.get(entry.nasPath);
      if (sz && pixPerDeg >= LAZY_PPD) {
        const corners = footprintCorners(entry, sz);
        const valid = corners.filter((c): c is [number, number] => c !== null);
        if (valid.length >= 3 && pointInPolygon(mx, my, valid)) { found = entry; break; }
      } else {
        const pt = project(entry.ra, entry.dec);
        if (pt) {
          const [px, py] = pt;
          if (Math.sqrt((mx - px) ** 2 + (my - py) ** 2) <= 6) { found = entry; break; }
        }
      }
    }
    hoveredEntry = found;
    hoverX = mx;
    hoverY = my;
  }

  function onMouseUp(e: MouseEvent) {
    const wasPanning = isPanning;
    isPanning = false;
    if (wasPanning && (Math.abs(e.clientX - panStartX) > 4 || Math.abs(e.clientY - panStartY) > 4)) return;

    // Empty space click is a no-op — images stay loaded
    if (!hoveredEntry) return;

    const idx = selectedEntries.indexOf(hoveredEntry);
    if (idx === -1) {
      // New frame — add to top of z-order
      selectedEntries = [...selectedEntries, hoveredEntry];
    } else if (idx === selectedEntries.length - 1) {
      // Already on top — deselect it
      selectedEntries = selectedEntries.filter((_, i) => i !== idx);
    } else {
      // Bring to top of z-order
      selectedEntries = [...selectedEntries.filter((_, i) => i !== idx), hoveredEntry];
    }
  }

  // ── Mount / resize ────────────────────────────────────────────────────────

  onMount(() => {
    const ro = new ResizeObserver((entries) => {
      canvasW = entries[0].contentRect.width  || canvasW;
      canvasH = entries[0].contentRect.height || canvasH;
    });
    ro.observe(container);
    canvasW = container.clientWidth;
    canvasH = container.clientHeight;

    // Load frames first so the atlas is usable; catalog loads in background
    GetAtlasIndex(rootPath)
      .then((indexResult) => {
        index = indexResult ?? [];
        if (index.length > 0) {
          viewRA    = index.reduce((s, f) => s + f.ra,  0) / index.length;
          viewDec   = index.reduce((s, f) => s + f.dec, 0) / index.length;
          pixPerDeg = canvasW / 30;
        }
        loading = false;
        scheduleLazyLoad();
        return GetCatalog();
      })
      .then((catalogResult) => {
        catalog = catalogResult ?? [];
      })
      .catch((e) => {
        loadError = String(e);
        loading = false;
      });

    return () => {
      ro.disconnect();
      if (lazyTimer) clearTimeout(lazyTimer);
    };
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="atlas-root" bind:this={container}>
  <canvas
    bind:this={canvas}
    class="atlas-canvas"
    width={canvasW}
    height={canvasH}
    onwheel={onWheel}
    onmousedown={onMouseDown}
    onmousemove={onMouseMove}
    onmouseup={onMouseUp}
    onmouseleave={() => { isPanning = false; hoveredEntry = null; }}
    style="cursor: {isPanning ? 'grabbing' : hoveredEntry ? 'pointer' : 'grab'};"
  ></canvas>

  <!-- Overlays rendered after canvas so they always paint on top -->
  {#if loading}
    <div class="atlas-overlay">
      <div class="atlas-spinner"></div>
      <span>Loading sky atlas…</span>
    </div>
  {:else if loadError}
    <div class="atlas-overlay atlas-error">{loadError}</div>
  {:else if index.length === 0}
    <div class="atlas-overlay atlas-empty">
      <div class="empty-icon">◎</div>
      <p>No stacked frames with sky coordinates found.</p>
      <p class="atlas-hint">Run <strong>Build Index</strong> to read WCS from FITS headers,<br>or <strong>✦ Analyze</strong> to plate-solve stacked frames.</p>
    </div>
  {/if}

  <!-- Loading throbber: frame sizes or preview images in flight -->
  {#if anyLoading}
    <div class="atlas-img-loading">
      <div class="atlas-spinner"></div>
      <span>{fetchingCount > 0 ? "Loading frames…" : "Loading preview…"}</span>
    </div>
  {/if}

  <!-- HUD -->
  <div class="atlas-hud">
    <span>RA {viewRA.toFixed(2)}°</span>
    <span>Dec {viewDec.toFixed(2)}°</span>
    <span>{(canvasW / pixPerDeg).toFixed(1)}° FOV</span>
    <span class="hud-sep">|</span>
    <span>{visibleIndex.length} frame{visibleIndex.length !== 1 ? "s" : ""}</span>
  </div>

  <!-- Frame type toggle -->
  <div class="atlas-toggle">
    <button
      class="toggle-btn"
      class:active={!showStacked}
      onclick={() => (showStacked = false)}
    >Processed</button>
    <button
      class="toggle-btn"
      class:active={showStacked}
      onclick={() => (showStacked = true)}
    >All</button>
  </div>

  <!-- Hover tooltip -->
  {#if hoveredEntry && !selectedEntries.includes(hoveredEntry)}
    <div
      class="atlas-tooltip"
      style="left: {Math.min(hoverX + 14, canvasW - 195)}px; top: {Math.min(hoverY - 10, canvasH - 80)}px;"
    >
      <div class="tt-name">{hoveredEntry.object || hoveredEntry.name}</div>
      <div class="tt-row">RA {hoveredEntry.ra.toFixed(3)}° · Dec {hoveredEntry.dec >= 0 ? "+" : ""}{hoveredEntry.dec.toFixed(3)}°</div>
      <div class="tt-row">{hoveredEntry.pixelScale.toFixed(2)} ″/px · {hoveredEntry.frameType}</div>
      <div class="tt-hint">Click to add / bring to front</div>
    </div>
  {/if}

  <!-- Side panel for most-recently selected frame -->
  {#if panelEntry}
    {@const sz = sizesVersion >= 0 ? sizes.get(panelEntry.nasPath) : undefined}
    <div class="atlas-panel">
      <div class="ap-header">
        <div class="ap-title">{panelEntry.object || panelEntry.name}</div>
        <button class="ap-close" onclick={() => (selectedEntries = [])}>✕</button>
      </div>

      <div class="ap-body">
        {#if selectedEntries.length > 1}
          <div class="ap-stack-hint">{selectedEntries.length} frames shown · showing latest</div>
        {/if}
        <div class="ap-row"><span class="ap-lbl">Type</span><span class="ap-val">{panelEntry.frameType}</span></div>
        <div class="ap-row"><span class="ap-lbl">RA</span><span class="ap-val">{panelEntry.ra.toFixed(4)}°</span></div>
        <div class="ap-row"><span class="ap-lbl">Dec</span><span class="ap-val">{panelEntry.dec >= 0 ? "+" : ""}{panelEntry.dec.toFixed(4)}°</span></div>
        <div class="ap-row"><span class="ap-lbl">Scale</span><span class="ap-val">{panelEntry.pixelScale.toFixed(2)} ″/px</span></div>
        <div class="ap-row">
          <span class="ap-lbl">Rotation</span>
          <span class="ap-val">{panelEntry.rotation.toFixed(1)}°</span>
        </div>
        {#if previewVersion >= 0 && previewImgs.has(panelEntry.nasPath)}
          {@const cur = rotOverVersion >= 0 ? (rotationOverrides.get(panelEntry.nasPath) ?? 0) : 0}
          <div class="ap-row ap-rot-row">
            <span class="ap-lbl">Adjust</span>
            <span class="ap-rot-btns">
              <button
                class="rot-btn"
                class:rot-active={cur === -90}
                title="Rotate 90° CCW"
                onclick={() => {
                  const path = panelEntry!.nasPath;
                  rotationOverrides.set(path, cur === -90 ? 0 : -90);
                  rotOverVersion++;
                }}
              >↺ 90°</button>
              <button
                class="rot-btn"
                class:rot-active={cur === 0}
                title="No adjustment"
                onclick={() => {
                  rotationOverrides.set(panelEntry!.nasPath, 0);
                  rotOverVersion++;
                }}
              >0°</button>
              <button
                class="rot-btn"
                class:rot-active={cur === 90}
                title="Rotate 90° CW"
                onclick={() => {
                  const path = panelEntry!.nasPath;
                  rotationOverrides.set(path, cur === 90 ? 0 : 90);
                  rotOverVersion++;
                }}
              >↻ 90°</button>
            </span>
          </div>
        {/if}
        {#if sz}
          <div class="ap-row"><span class="ap-lbl">Size</span><span class="ap-val">{sz.width} × {sz.height} px</span></div>
          <div class="ap-row">
            <span class="ap-lbl">FOV</span>
            <span class="ap-val">
              {((sz.width  * panelEntry.pixelScale) / 3600).toFixed(2)}° ×
              {((sz.height * panelEntry.pixelScale) / 3600).toFixed(2)}°
            </span>
          </div>
        {/if}
        <div class="ap-row ap-file-row"><span class="ap-lbl">File</span><span class="ap-val ap-file">{panelEntry.name}</span></div>
      </div>

      {#if onframeopen}
        <button class="ap-open-btn" onclick={() => onframeopen!(panelEntry!.nasPath)}>
          Open in Library →
        </button>
      {/if}
    </div>
  {/if}

  <div class="atlas-help">Scroll to zoom · Drag to pan · Click frames to overlay · Click again to bring to front</div>
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
    /* canvas sits below all overlay UI */
    z-index: 0;
  }

  .atlas-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
    font-size: 0.875rem;
    gap: 12px;
    z-index: 20;
    pointer-events: none;
    background: rgba(5, 6, 16, 0.75);
  }
  .atlas-error { color: var(--danger); }
  .atlas-empty { gap: 6px; text-align: center; }
  .empty-icon  { font-size: 2.5rem; color: var(--accent-dim); }
  .atlas-hint  { font-size: 0.8rem; color: var(--text-secondary); opacity: 0.7; line-height: 1.5; }

  .atlas-spinner {
    width: 32px;
    height: 32px;
    border: 3px solid color-mix(in srgb, var(--accent) 25%, transparent);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }

  /* Per-image loading indicator (bottom-left corner) */
  .atlas-img-loading {
    position: absolute;
    bottom: 32px;
    left: 10px;
    display: flex;
    align-items: center;
    gap: 8px;
    background: color-mix(in srgb, var(--bg-panel) 90%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 5px 10px;
    font-size: 0.72rem;
    color: var(--text-secondary);
    z-index: 25;
    pointer-events: none;
  }
  .atlas-img-loading .atlas-spinner {
    width: 14px;
    height: 14px;
    border-width: 2px;
    flex-shrink: 0;
  }

  /* HUD */
  .atlas-hud {
    position: absolute;
    top: 8px;
    left: 10px;
    display: flex;
    gap: 10px;
    font-size: 0.72rem;
    font-family: "Consolas", "Fira Code", monospace;
    color: var(--text-secondary);
    pointer-events: none;
    background: color-mix(in srgb, var(--bg-panel) 80%, transparent);
    padding: 3px 8px;
    border-radius: 4px;
    border: 1px solid var(--border);
  }
  .hud-sep { opacity: 0.4; }

  /* Frame type toggle */
  .atlas-toggle {
    position: absolute;
    top: 8px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    background: color-mix(in srgb, var(--bg-panel) 88%, transparent);
    border: 1px solid var(--border);
    border-radius: 5px;
    overflow: hidden;
    z-index: 20;
  }
  .toggle-btn {
    padding: 3px 12px;
    font-size: 0.72rem;
    color: var(--text-secondary);
    background: none;
    border: none;
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .toggle-btn.active {
    background: color-mix(in srgb, var(--accent-dim) 40%, transparent);
    color: var(--accent);
  }
  .toggle-btn:hover:not(.active) {
    background: color-mix(in srgb, var(--bg-panel) 60%, transparent);
    color: var(--text-primary);
  }

  /* Hover tooltip */
  .atlas-tooltip {
    position: absolute;
    background: color-mix(in srgb, var(--bg-panel) 92%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 5px;
    padding: 6px 10px;
    pointer-events: none;
    z-index: 20;
    min-width: 175px;
  }
  .tt-name { font-size: 0.82rem; font-weight: 600; color: var(--text-primary); margin-bottom: 3px; }
  .tt-row  { font-size: 0.72rem; color: var(--text-secondary); font-family: "Consolas", monospace; }
  .tt-hint { font-size: 0.68rem; color: var(--accent); margin-top: 4px; font-style: italic; }

  /* Selected-frame panel */
  .atlas-panel {
    position: absolute;
    top: 8px;
    right: 8px;
    width: 250px;
    background: color-mix(in srgb, var(--bg-panel) 95%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 7px;
    z-index: 30;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    max-height: calc(100% - 16px);
  }
  .ap-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px 6px;
    border-bottom: 1px solid var(--border);
    gap: 6px;
    flex-shrink: 0;
  }
  .ap-title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text-primary);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ap-close {
    flex-shrink: 0;
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 0.8rem;
    padding: 0 2px;
    line-height: 1;
    opacity: 0.6;
  }
  .ap-close:hover { opacity: 1; color: var(--text-primary); }

  .ap-body {
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    overflow-y: auto;
  }
  .ap-stack-hint {
    font-size: 0.68rem;
    color: var(--accent);
    font-style: italic;
    margin-bottom: 4px;
    opacity: 0.8;
  }
  .ap-row {
    display: flex;
    justify-content: space-between;
    gap: 6px;
    font-size: 0.75rem;
  }
  .ap-lbl {
    color: var(--text-secondary);
    flex-shrink: 0;
    font-family: "Consolas", monospace;
  }
  .ap-val {
    color: var(--text-primary);
    font-family: "Consolas", monospace;
    text-align: right;
  }
  .ap-file-row { margin-top: 4px; }
  .ap-file {
    font-size: 0.68rem;
    word-break: break-all;
    text-align: right;
    opacity: 0.65;
  }

  /* Rotation override row */
  .ap-rot-row { align-items: center; margin-top: 2px; }
  .ap-rot-btns {
    display: flex;
    gap: 3px;
  }
  .rot-btn {
    padding: 2px 7px;
    font-size: 0.68rem;
    font-family: "Consolas", monospace;
    background: color-mix(in srgb, var(--bg-panel) 60%, transparent);
    border: 1px solid var(--border);
    border-radius: 3px;
    color: var(--text-secondary);
    cursor: pointer;
    transition: background 0.12s, color 0.12s, border-color 0.12s;
  }
  .rot-btn:hover { border-color: var(--border-accent); color: var(--text-primary); }
  .rot-btn.rot-active {
    background: color-mix(in srgb, var(--accent-dim) 40%, transparent);
    border-color: var(--accent);
    color: var(--accent);
  }
  .ap-open-btn {
    flex-shrink: 0;
    margin: 6px 10px 10px;
    padding: 6px 0;
    background: color-mix(in srgb, var(--accent-dim) 30%, transparent);
    border: 1px solid var(--border-accent);
    border-radius: 4px;
    color: var(--accent);
    font-size: 0.78rem;
    cursor: pointer;
    transition: background 0.15s, border-color 0.15s;
  }
  .ap-open-btn:hover {
    background: color-mix(in srgb, var(--accent-dim) 55%, transparent);
    border-color: var(--accent);
  }

  .atlas-help {
    position: absolute;
    bottom: 8px;
    right: 10px;
    font-size: 0.68rem;
    color: var(--text-secondary);
    opacity: 0.45;
    pointer-events: none;
  }
</style>
