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

  // Size cache: nasPath → pixel dimensions (plain Map; sizesVersion drives redraws)
  const sizes = new Map<string, { width: number; height: number }>();
  let sizesVersion = $state(0);
  const fetchingPaths = new Set<string>();

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
  let selectedEntry = $state<app.AtlasIndexEntry | null>(null);

  // Canvas preview image for selected frame
  let previewImg     = $state<HTMLImageElement | null>(null);
  let previewLoading = $state(false);

  let lazyTimer: ReturnType<typeof setTimeout> | null = null;
  const LAZY_PPD = 8;

  // ── Preview loading ───────────────────────────────────────────────────────

  $effect(() => {
    if (!selectedEntry) {
      previewImg = null;
      previewLoading = false;
      return;
    }
    const path = selectedEntry.nasPath;
    previewLoading = true;
    previewImg = null;
    GeneratePreview(path, 2)
      .then((url) => {
        if (selectedEntry?.nasPath !== path) return;
        const img = new Image();
        img.onload = () => { if (selectedEntry?.nasPath === path) { previewImg = img; previewLoading = false; } };
        img.onerror = () => { if (selectedEntry?.nasPath === path) previewLoading = false; };
        img.src = url;
      })
      .catch(() => { if (selectedEntry?.nasPath === path) previewLoading = false; });
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

  function isOnScreen(corners: ([number, number] | null)[]): boolean {
    const m = 80;
    return corners.some(
      (c) => c !== null && c[0] >= -m && c[0] <= canvasW + m && c[1] >= -m && c[1] <= canvasH + m,
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
    ctx.fillStyle = "rgba(5,6,16,0.7)";
    ctx.fillRect(x - 2, y - 10, tw + 4, 13);
    ctx.fillStyle = saved;
    ctx.fillText(text, x, y);
  }

  function drawGrid(ctx: CanvasRenderingContext2D) {
    ctx.save();
    ctx.strokeStyle = "rgba(80,100,140,0.28)";
    ctx.lineWidth = 0.7;
    ctx.setLineDash([4, 6]);
    ctx.font = "9px monospace";

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
    ctx.fillStyle = "rgba(155,180,230,0.92)";
    for (let ra = 0; ra < 360; ra += raStep) {
      const lp = project(ra, viewDec);
      if (lp && lp[0] >= 20 && lp[0] <= canvasW - 20 && lp[1] >= 14 && lp[1] <= canvasH - 4) {
        labelBg(ctx, `${ra}°`, lp[0] + 2, lp[1] - 2);
      }
    }

    // Dec labels (teal-ish) at current RA centre
    ctx.fillStyle = "rgba(110,215,190,0.92)";
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
    for (const entry of index) {
      const cp = project(entry.ra, entry.dec);
      if (!cp) continue;
      const [cx, cy] = cp;
      if (cx < -100 || cx > canvasW + 100 || cy < -100 || cy > canvasH + 100) continue;

      const sz    = sizes.get(entry.nasPath);
      const isSel = entry === selectedEntry;
      const isHov = entry === hoveredEntry;

      if (sz && pixPerDeg >= LAZY_PPD) {
        const corners = footprintCorners(entry, sz);
        if (!isOnScreen(corners)) continue;
        const valid = corners.filter((c): c is [number, number] => c !== null);
        if (valid.length < 3) continue;

        if (isSel && previewImg) {
          // Draw the actual image aligned to the footprint
          const wPx = sz.width  * entry.pixelScale / 3600 * pixPerDeg;
          const hPx = sz.height * entry.pixelScale / 3600 * pixPerDeg;
          ctx.save();
          ctx.translate(cx, cy);
          ctx.rotate(-entry.rotation * Math.PI / 180);
          ctx.drawImage(previewImg, -wPx / 2, -hPx / 2, wPx, hPx);
          ctx.restore();
          // Golden border over the image
          ctx.beginPath();
          ctx.moveTo(valid[0][0], valid[0][1]);
          for (let i = 1; i < valid.length; i++) ctx.lineTo(valid[i][0], valid[i][1]);
          ctx.closePath();
          ctx.strokeStyle = "rgba(255,210,80,0.9)";
          ctx.lineWidth = 2;
          ctx.stroke();
        } else if (isSel && previewLoading) {
          // Footprint with pulsing style while image loads
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
          // Normal footprint
          ctx.beginPath();
          ctx.moveTo(valid[0][0], valid[0][1]);
          for (let i = 1; i < valid.length; i++) ctx.lineTo(valid[i][0], valid[i][1]);
          ctx.closePath();
          ctx.fillStyle   = isHov ? "rgba(100,190,255,0.22)" : "rgba(80,140,220,0.10)";
          ctx.strokeStyle = isHov ? "rgba(120,210,255,0.95)" : "rgba(100,160,255,0.55)";
          ctx.lineWidth   = isHov ? 2 : 1.2;
          ctx.fill();
          ctx.stroke();
          ctx.fillStyle = isHov ? "rgba(190,230,255,1)" : "rgba(160,205,255,0.85)";
          ctx.font      = isHov ? "bold 11px monospace" : "10px monospace";
          ctx.textAlign = "center";
          ctx.fillText(entry.object || entry.name, cx, cy + 4);
          ctx.textAlign = "left";
        }
      } else {
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
    ctx.restore();
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
    void hoveredEntry; void selectedEntry; void canvasW; void canvasH; void sizesVersion;
    void previewImg; void previewLoading;
    requestAnimationFrame(redraw);
  });

  // ── Lazy size loading ─────────────────────────────────────────────────────

  function scheduleLazyLoad() {
    if (lazyTimer) clearTimeout(lazyTimer);
    lazyTimer = setTimeout(checkLazyLoad, 300);
  }

  function checkLazyLoad() {
    if (pixPerDeg < LAZY_PPD) return;
    for (const entry of index) {
      if (sizes.has(entry.nasPath) || fetchingPaths.has(entry.nasPath)) continue;
      const pt = project(entry.ra, entry.dec);
      if (!pt) continue;
      const [x, y] = pt;
      if (x < -200 || x > canvasW + 200 || y < -200 || y > canvasH + 200) continue;
      fetchingPaths.add(entry.nasPath);
      GetAtlasFrameSize(entry.nasPath)
        .then((sz) => {
          fetchingPaths.delete(entry.nasPath);
          if (sz.width > 0 && sz.height > 0) {
            sizes.set(entry.nasPath, { width: sz.width, height: sz.height });
            sizesVersion++;
          }
        })
        .catch(() => { fetchingPaths.delete(entry.nasPath); });
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

    // Keep the sky point under the cursor fixed after zoom
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
      // Positive dx (drag right) → viewRA increases → sky shifts eastward (left)
      // which feels like "the sky follows your hand" toward the right
      viewRA  = panStartRA  + dx / pixPerDeg / Math.cos(viewDec * Math.PI / 180);
      viewDec = Math.max(-89, Math.min(89, panStartDec + dy / pixPerDeg));
      hoveredEntry = null;
      scheduleLazyLoad();
      return;
    }

    // Hit-test footprint polygon (if size loaded) or dot circle
    let found: app.AtlasIndexEntry | null = null;
    for (const entry of index) {
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
    selectedEntry = hoveredEntry && hoveredEntry !== selectedEntry ? hoveredEntry : null;
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

    Promise.all([GetAtlasIndex(rootPath), GetCatalog()])
      .then(([indexResult, catalogResult]) => {
        index   = indexResult  ?? [];
        catalog = catalogResult ?? [];
        if (index.length > 0) {
          viewRA    = index.reduce((s, f) => s + f.ra,  0) / index.length;
          viewDec   = index.reduce((s, f) => s + f.dec, 0) / index.length;
          pixPerDeg = canvasW / 30;
        }
        scheduleLazyLoad();
      })
      .catch((e) => { loadError = String(e); })
      .finally(() => { loading = false; });

    return () => {
      ro.disconnect();
      if (lazyTimer) clearTimeout(lazyTimer);
    };
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="atlas-root" bind:this={container}>
  {#if loading}
    <div class="atlas-overlay">
      <span class="atlas-spinner">◌</span> Loading sky atlas…
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

  <!-- HUD -->
  <div class="atlas-hud">
    <span>RA {viewRA.toFixed(2)}°</span>
    <span>Dec {viewDec.toFixed(2)}°</span>
    <span>{(canvasW / pixPerDeg).toFixed(1)}° FOV</span>
    <span class="hud-sep">|</span>
    <span>{index.length} frame{index.length !== 1 ? "s" : ""}</span>
  </div>

  <!-- Hover tooltip (hidden when a frame is selected) -->
  {#if hoveredEntry && hoveredEntry !== selectedEntry}
    <div
      class="atlas-tooltip"
      style="left: {Math.min(hoverX + 14, canvasW - 195)}px; top: {Math.min(hoverY - 10, canvasH - 80)}px;"
    >
      <div class="tt-name">{hoveredEntry.object || hoveredEntry.name}</div>
      <div class="tt-row">RA {hoveredEntry.ra.toFixed(3)}° · Dec {hoveredEntry.dec >= 0 ? "+" : ""}{hoveredEntry.dec.toFixed(3)}°</div>
      <div class="tt-row">{hoveredEntry.pixelScale.toFixed(2)} ″/px · {hoveredEntry.frameType}</div>
      <div class="tt-hint">Click to inspect</div>
    </div>
  {/if}

  <!-- Selected-frame side panel -->
  {#if selectedEntry}
    {@const sz = sizesVersion >= 0 ? sizes.get(selectedEntry.nasPath) : undefined}
    <div class="atlas-panel">
      <div class="ap-header">
        <div class="ap-title">{selectedEntry.object || selectedEntry.name}</div>
        <button class="ap-close" onclick={() => (selectedEntry = null)}>✕</button>
      </div>

      <div class="ap-body">
        <div class="ap-row"><span class="ap-lbl">Type</span><span class="ap-val">{selectedEntry.frameType}</span></div>
        <div class="ap-row"><span class="ap-lbl">RA</span><span class="ap-val">{selectedEntry.ra.toFixed(4)}°</span></div>
        <div class="ap-row"><span class="ap-lbl">Dec</span><span class="ap-val">{selectedEntry.dec >= 0 ? "+" : ""}{selectedEntry.dec.toFixed(4)}°</span></div>
        <div class="ap-row"><span class="ap-lbl">Scale</span><span class="ap-val">{selectedEntry.pixelScale.toFixed(2)} ″/px</span></div>
        {#if sz}
          <div class="ap-row"><span class="ap-lbl">Size</span><span class="ap-val">{sz.width} × {sz.height} px</span></div>
          <div class="ap-row">
            <span class="ap-lbl">FOV</span>
            <span class="ap-val">
              {((sz.width  * selectedEntry.pixelScale) / 3600).toFixed(2)}° ×
              {((sz.height * selectedEntry.pixelScale) / 3600).toFixed(2)}°
            </span>
          </div>
        {/if}
        <div class="ap-row ap-file-row"><span class="ap-lbl">File</span><span class="ap-val ap-file">{selectedEntry.name}</span></div>
      </div>

      {#if onframeopen}
        <button class="ap-open-btn" onclick={() => onframeopen!(selectedEntry!.nasPath)}>
          Open in Library →
        </button>
      {/if}
    </div>
  {/if}

  <div class="atlas-help">Scroll to zoom · Drag to pan · Click frame to inspect</div>
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
    gap: 8px;
    z-index: 10;
    pointer-events: none;
  }
  .atlas-error { color: var(--danger); }
  .atlas-empty { gap: 6px; text-align: center; }
  .empty-icon  { font-size: 2.5rem; color: var(--accent-dim); }
  .atlas-hint  { font-size: 0.8rem; color: var(--text-secondary); opacity: 0.7; line-height: 1.5; }

  .atlas-spinner {
    display: inline-block;
    animation: spin 1.2s linear infinite;
    color: var(--accent);
    font-size: 1.4rem;
    margin-bottom: 4px;
  }
  @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

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
