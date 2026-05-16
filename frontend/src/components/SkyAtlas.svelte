<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import type { app } from "../../wailsjs/go/models";
  import { GetAtlasFrames, GetCatalog } from "../../wailsjs/go/app/App.js";

  interface Props {
    rootPath: string;
    onframeclick?: (nasPath: string) => void;
  }

  let { rootPath, onframeclick }: Props = $props();

  // ── State ─────────────────────────────────────────────────────────────────
  let canvas: HTMLCanvasElement;
  let container: HTMLElement;
  let canvasW = $state(800);
  let canvasH = $state(600);

  let viewRA  = $state(180);  // center RA (degrees)
  let viewDec = $state(0);    // center Dec (degrees)
  let pixPerDeg = $state(12); // zoom: pixels per degree

  let frames = $state<app.AtlasFrame[]>([]);
  let catalog = $state<app.CatalogObject[]>([]);
  let loading = $state(true);
  let loadError = $state("");

  // Pan state
  let isPanning = $state(false);
  let panStartX = 0;
  let panStartY = 0;
  let panStartRA = 0;
  let panStartDec = 0;

  // Hover info
  let hoveredFrame = $state<app.AtlasFrame | null>(null);
  let hoverX = $state(0);
  let hoverY = $state(0);

  // ── Projection ────────────────────────────────────────────────────────────

  // Gnomonic projection: sky (ra,dec) → canvas (x,y). Returns null if behind plane.
  // Convention: east = left (astronomical), north = up.
  function project(ra: number, dec: number): [number, number] | null {
    const ra0  = viewRA  * Math.PI / 180;
    const dec0 = viewDec * Math.PI / 180;
    const raR  = ra  * Math.PI / 180;
    const decR = dec * Math.PI / 180;

    const dRA  = raR - ra0;
    const denom = Math.sin(dec0) * Math.sin(decR) + Math.cos(dec0) * Math.cos(decR) * Math.cos(dRA);
    if (denom <= 0.001) return null;

    const xi  = Math.cos(decR) * Math.sin(dRA) / denom;
    const eta = (Math.cos(dec0) * Math.sin(decR) - Math.sin(dec0) * Math.cos(decR) * Math.cos(dRA)) / denom;

    // xi positive = east = left; eta positive = north = up
    const xDeg = xi  * 180 / Math.PI;
    const yDeg = eta * 180 / Math.PI;

    return [
      canvasW / 2 - xDeg * pixPerDeg,
      canvasH / 2 - yDeg * pixPerDeg,
    ];
  }

  // Inverse gnomonic: canvas (x,y) → sky (ra, dec in degrees).
  function unproject(x: number, y: number): [number, number] {
    const xDeg = -(x - canvasW / 2) / pixPerDeg; // east=left so flip
    const yDeg = -(y - canvasH / 2) / pixPerDeg; // north=up so flip
    const xi  = xDeg * Math.PI / 180;
    const eta = yDeg * Math.PI / 180;

    const dec0 = viewDec * Math.PI / 180;
    const ra0  = viewRA  * Math.PI / 180;

    const rho = Math.sqrt(xi * xi + eta * eta);
    if (rho < 1e-10) return [viewRA, viewDec];

    const c   = Math.atan(rho);
    const dec = Math.asin(Math.cos(c) * Math.sin(dec0) + (eta * Math.sin(c) * Math.cos(dec0)) / rho);
    const ra  = ra0 + Math.atan2(xi * Math.sin(c), rho * Math.cos(dec0) * Math.cos(c) - eta * Math.sin(dec0) * Math.sin(c));

    return [((ra * 180 / Math.PI) + 360) % 360, dec * 180 / Math.PI];
  }

  // ── Footprint helpers ─────────────────────────────────────────────────────

  // Returns the 4 canvas-space corners of a frame's sky footprint.
  function footprintCorners(f: app.AtlasFrame): ([number, number] | null)[] {
    const scaleDeg = f.pixelScale / 3600;
    const hw = (f.width  / 2) * scaleDeg;
    const hh = (f.height / 2) * scaleDeg;
    const θ  = f.rotation * Math.PI / 180;
    const cosDec = Math.cos(f.dec * Math.PI / 180);

    return (
      [[-hw, -hh], [hw, -hh], [hw, hh], [-hw, hh]] as [number, number][]
    ).map(([dx, dy]) => {
      const rx = dx * Math.cos(θ) - dy * Math.sin(θ);
      const ry = dx * Math.sin(θ) + dy * Math.cos(θ);
      return project(f.ra + rx / cosDec, f.dec + ry);
    });
  }

  function isOnScreen(corners: ([number, number] | null)[]): boolean {
    const margin = 60;
    return corners.some(
      (c) => c !== null && c[0] >= -margin && c[0] <= canvasW + margin && c[1] >= -margin && c[1] <= canvasH + margin,
    );
  }

  // Point-in-polygon test for frame hit-testing (canvas coords).
  function pointInPolygon(px: number, py: number, pts: [number, number][]): boolean {
    let inside = false;
    for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
      const xi = pts[i][0], yi = pts[i][1];
      const xj = pts[j][0], yj = pts[j][1];
      const intersect = yi > py !== yj > py && px < ((xj - xi) * (py - yi)) / (yj - yi) + xi;
      if (intersect) inside = !inside;
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
    drawFootprints(ctx);
    drawCompass(ctx);
  }

  function drawGrid(ctx: CanvasRenderingContext2D) {
    ctx.save();
    ctx.strokeStyle = "rgba(80,100,140,0.25)";
    ctx.lineWidth = 0.7;
    ctx.setLineDash([4, 6]);
    ctx.font = "9px monospace";
    ctx.fillStyle = "rgba(120,150,200,0.5)";

    // Dec lines every 10°
    const decStep = pixPerDeg >= 30 ? 5 : 10;
    for (let dec = -90; dec <= 90; dec += decStep) {
      const points: [number, number][] = [];
      for (let ra = 0; ra <= 360; ra += 2) {
        const p = project(ra, dec);
        if (p) points.push(p);
      }
      if (points.length < 2) continue;
      ctx.beginPath();
      ctx.moveTo(points[0][0], points[0][1]);
      for (let i = 1; i < points.length; i++) ctx.lineTo(points[i][0], points[i][1]);
      ctx.stroke();
    }

    // RA lines every 15° (1h)
    const raStep = pixPerDeg >= 50 ? 5 : 15;
    for (let ra = 0; ra < 360; ra += raStep) {
      const points: [number, number][] = [];
      for (let dec = -85; dec <= 85; dec += 2) {
        const p = project(ra, dec);
        if (p) points.push(p);
      }
      if (points.length < 2) continue;
      ctx.beginPath();
      ctx.moveTo(points[0][0], points[0][1]);
      for (let i = 1; i < points.length; i++) ctx.lineTo(points[i][0], points[i][1]);
      ctx.stroke();

      // Label at Dec=0
      const labelPt = project(ra, viewDec);
      if (labelPt && labelPt[0] >= 20 && labelPt[0] <= canvasW - 20) {
        ctx.fillText(`${ra}°`, labelPt[0] + 2, labelPt[1] - 3);
      }
    }

    ctx.setLineDash([]);
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
        // Star size proportional to brightness
        const r = Math.max(0.8, Math.min(3.5, (4 - obj.mag) * 0.6));
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.fillStyle = "rgba(220,230,255,0.85)";
        ctx.fill();
        // Label for bright stars
        if (obj.mag < 2.5 || pixPerDeg > 60) {
          ctx.fillStyle = "rgba(180,200,255,0.7)";
          ctx.font = "9px monospace";
          ctx.fillText(obj.name, x + r + 2, y + 3);
        }
      } else {
        // DSO: crosshair
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

  function drawFootprints(ctx: CanvasRenderingContext2D) {
    ctx.save();
    for (const f of frames) {
      const corners = footprintCorners(f);
      if (!isOnScreen(corners)) continue;
      const valid = corners.filter((c): c is [number, number] => c !== null);
      if (valid.length < 3) continue;

      const isHovered = f === hoveredFrame;

      ctx.beginPath();
      ctx.moveTo(valid[0][0], valid[0][1]);
      for (let i = 1; i < valid.length; i++) ctx.lineTo(valid[i][0], valid[i][1]);
      ctx.closePath();

      ctx.fillStyle   = isHovered ? "rgba(100,180,255,0.22)" : "rgba(80,140,220,0.10)";
      ctx.fill();
      ctx.strokeStyle = isHovered ? "rgba(120,200,255,0.95)" : "rgba(100,160,255,0.55)";
      ctx.lineWidth   = isHovered ? 2 : 1.2;
      ctx.stroke();

      // Label at center
      const cp = project(f.ra, f.dec);
      if (cp) {
        const label = f.object || f.name;
        ctx.fillStyle = isHovered ? "rgba(180,225,255,1)" : "rgba(160,200,255,0.85)";
        ctx.font = isHovered ? "bold 11px monospace" : "10px monospace";
        ctx.textAlign = "center";
        ctx.fillText(label, cp[0], cp[1] + 4);
        ctx.textAlign = "left";
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
    void viewRA; void viewDec; void pixPerDeg; void frames; void catalog; void hoveredFrame;
    void canvasW; void canvasH;
    requestAnimationFrame(redraw);
  });

  // ── Interaction ───────────────────────────────────────────────────────────

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.18 : 0.847;
    const rect = canvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    const [skyRA, skyDec] = unproject(mx, my);

    pixPerDeg = Math.max(0.3, Math.min(8000, pixPerDeg * factor));

    // Keep the point under the mouse fixed
    const [nx, ny] = project(skyRA, skyDec) ?? [canvasW / 2, canvasH / 2];
    viewRA  = viewRA  + (mx - nx) / pixPerDeg * (-1) / Math.cos(viewDec * Math.PI / 180);
    viewDec = viewDec + (my - ny) / pixPerDeg;
  }

  function onMouseDown(e: MouseEvent) {
    if (e.button !== 0) return;
    isPanning = true;
    panStartX = e.clientX;
    panStartY = e.clientY;
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
      // Dragging right reveals western sky (lower RA since east=left)
      viewRA  = panStartRA  - dx / pixPerDeg / Math.cos(viewDec * Math.PI / 180);
      viewDec = panStartDec - dy / pixPerDeg;
      viewDec = Math.max(-89, Math.min(89, viewDec));
      hoveredFrame = null;
      return;
    }

    // Hit-test footprints for hover tooltip
    let found: app.AtlasFrame | null = null;
    for (const f of frames) {
      const corners = footprintCorners(f);
      const valid = corners.filter((c): c is [number, number] => c !== null);
      if (valid.length >= 3 && pointInPolygon(mx, my, valid)) {
        found = f;
        break;
      }
    }
    hoveredFrame = found;
    hoverX = mx;
    hoverY = my;
  }

  function onMouseUp() { isPanning = false; }

  function onClick(e: MouseEvent) {
    if (!hoveredFrame || !onframeclick) return;
    onframeclick(hoveredFrame.nasPath);
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

    Promise.all([GetAtlasFrames(rootPath), GetCatalog()])
      .then(([framesResult, catalogResult]) => {
        frames  = framesResult  ?? [];
        catalog = catalogResult ?? [];

        if (frames.length > 0) {
          viewRA  = frames.reduce((s, f) => s + f.ra,  0) / frames.length;
          viewDec = frames.reduce((s, f) => s + f.dec, 0) / frames.length;
          pixPerDeg = canvasW / 30;
        }
      })
      .catch((e) => { loadError = String(e); })
      .finally(() => { loading = false; });

    return () => ro.disconnect();
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
  {:else if frames.length === 0}
    <div class="atlas-overlay atlas-empty">
      <div class="empty-icon">◎</div>
      <p>No images with sky coordinates found.</p>
      <p class="atlas-hint">Run <strong>✦ Analyze</strong> on stacked frames to plate-solve them.</p>
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
    onmouseleave={onMouseUp}
    onclick={onClick}
    style="cursor: {isPanning ? 'grabbing' : hoveredFrame ? 'pointer' : 'grab'};"
  ></canvas>

  <!-- HUD: coordinates + zoom -->
  <div class="atlas-hud">
    <span>RA {viewRA.toFixed(2)}°</span>
    <span>Dec {viewDec.toFixed(2)}°</span>
    <span>{(canvasW / pixPerDeg).toFixed(1)}° FOV</span>
    <span class="hud-sep">|</span>
    <span>{frames.length} frame{frames.length !== 1 ? "s" : ""}</span>
  </div>

  <!-- Hover tooltip -->
  {#if hoveredFrame}
    <div
      class="atlas-tooltip"
      style="left: {Math.min(hoverX + 14, canvasW - 180)}px; top: {Math.min(hoverY - 10, canvasH - 90)}px;"
    >
      <div class="tt-name">{hoveredFrame.object || hoveredFrame.name}</div>
      <div class="tt-row">RA {hoveredFrame.ra.toFixed(3)}°</div>
      <div class="tt-row">Dec {hoveredFrame.dec.toFixed(3)}°</div>
      <div class="tt-row">{hoveredFrame.pixelScale.toFixed(2)} ″/px · {hoveredFrame.frameType}</div>
      {#if onframeclick}
        <div class="tt-hint">Click to open</div>
      {/if}
    </div>
  {/if}

  <!-- Control help -->
  <div class="atlas-help">Scroll to zoom · Drag to pan</div>
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

  .atlas-empty { gap: 6px; }
  .empty-icon { font-size: 2.5rem; color: var(--accent-dim); }
  .atlas-hint { font-size: 0.8rem; color: var(--text-secondary); opacity: 0.7; }

  .atlas-spinner {
    display: inline-block;
    animation: spin 1.2s linear infinite;
    color: var(--accent);
    font-size: 1.4rem;
    margin-bottom: 4px;
  }
  @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

  .atlas-hud {
    position: absolute;
    top: 8px;
    left: 10px;
    display: flex;
    gap: 10px;
    font-size: 0.72rem;
    font-family: "Consolas", "Fira Code", monospace;
    color: rgba(140, 170, 220, 0.75);
    pointer-events: none;
    background: rgba(5, 6, 16, 0.55);
    padding: 3px 8px;
    border-radius: 4px;
  }
  .hud-sep { opacity: 0.4; }

  .atlas-tooltip {
    position: absolute;
    background: rgba(12, 14, 28, 0.9);
    border: 1px solid rgba(100, 160, 255, 0.5);
    border-radius: 5px;
    padding: 7px 10px;
    pointer-events: none;
    z-index: 20;
    min-width: 160px;
    max-width: 200px;
  }
  .tt-name { font-size: 0.82rem; font-weight: 600; color: rgba(200, 225, 255, 1); margin-bottom: 4px; }
  .tt-row  { font-size: 0.75rem; color: rgba(160, 185, 220, 0.85); font-family: "Consolas", monospace; }
  .tt-hint { font-size: 0.7rem; color: var(--accent); margin-top: 4px; font-style: italic; }

  .atlas-help {
    position: absolute;
    bottom: 8px;
    right: 10px;
    font-size: 0.68rem;
    color: rgba(100, 130, 180, 0.5);
    pointer-events: none;
  }
</style>
