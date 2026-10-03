<script lang="ts">
  import { untrack } from "svelte";
  import type * as app from "$models/app";
  import type * as fits from "$models/fits";
  import type * as catalog from "$models/catalog";
  import { LoadRasterImage, GetAnnotations } from "$app";
  import { basicRows, advancedRows, formatRA, formatDec } from "../lib/utils";
  import { computeStretch } from "../lib/stretchPreview";
  import { previewPrefs as pp } from "../lib/previewPrefs.svelte";
  import {
    loadPreview,
    peekPreview,
    prefetchPreview,
    type DecodedPreview,
    type HistBins,
  } from "../lib/library/previewCache";

  interface Props {
    entry: app.EnrichedFileEntry;
    qualityFrame?: app.LibraryFrame | null;
    onclose: () => void;
    /** Title-bar navigation; the button is shown disabled when absent but its sibling exists. */
    onprev?: () => void;
    onnext?: () => void;
    /** Title-bar actions — each button renders only when its handler is provided. */
    onreject?: () => void;
    onrestore?: () => void;
    ondelete?: () => void;
    onreveal?: () => void;
    onblink?: () => void;
    blinkTitle?: string;
    /** Path of the frame likely to be shown next; warmed into the preview cache. */
    prefetch?: string | null;
  }

  let {
    entry,
    qualityFrame = null,
    onclose,
    onprev,
    onnext,
    onreject,
    onrestore,
    ondelete,
    onreveal,
    onblink,
    blinkTitle = "Blink (b)",
    prefetch = null,
  }: Props = $props();

  let isRejected = $derived(qualityFrame?.isRejected ?? entry.isRejected);

  function isRasterFile(path: string): boolean {
    return path.toLowerCase().endsWith(".png");
  }

  // Primitive derived: a new entry object for the same file must not retrigger loading.
  let entryPath = $derived(entry.path);
  let isRaster = $derived(isRasterFile(entryPath));

  let isProcessed = $derived(qualityFrame?.frameType === "processed");

  let statsCollapsed = $state(false);
  let coordsCollapsed = $state(false);

  let channelMode = $state<0 | 1 | 2 | 3>(0);

  let showHistogram = $state(false);
  let histCanvas = $state<HTMLCanvasElement | null>(null);
  let histBins = $state<HistBins | null>(null);

  let showAnnotations = $state(false);
  let annotations = $state<catalog.Annotation[]>([]);

  // ── Viewport size tracking ─────────────────────────────────────────────────
  let viewportEl = $state<HTMLElement | null>(null);
  let viewportW = $state(0);
  let viewportH = $state(0);

  $effect(() => {
    if (!viewportEl) return;
    const obs = new ResizeObserver((entries) => {
      viewportW = entries[0].contentRect.width;
      viewportH = entries[0].contentRect.height;
    });
    obs.observe(viewportEl);
    viewportW = viewportEl.clientWidth;
    viewportH = viewportEl.clientHeight;
    return () => obs.disconnect();
  });

  let previewLoading = $state(false);
  let previewError = $state("");
  let fitsHeader = $state<fits.FITSHeader | null>(null);
  let previewReqId = 0;
  let hasImage = $state(false);
  let rasterDataUrl = $state("");

  // ── Zoom / pan ────────────────────────────────────────────────────────────
  let zoom = $state(1);
  let panX = $state(0);
  let panY = $state(0);
  let isPanning = $state(false);
  let panStartX = 0;
  let panStartY = 0;

  function resetView() {
    zoom = 1;
    panX = 0;
    panY = 0;
    scheduleRender();
  }

  // Offscreen WebGL canvas — autostretch rendered here; displayCanvas blits it via ctx2d.drawImage.
  let glCanvas: HTMLCanvasElement | null = null;
  let gl: WebGL2RenderingContext | null = null;
  let program: WebGLProgram | null = null;
  let glTex: WebGLTexture | null = null;
  let glU: {
    uTex: WebGLUniformLocation;
    uGain: WebGLUniformLocation;
    uShadow: WebGLUniformLocation;
    uMidtone: WebGLUniformLocation;
    uLinear: WebGLUniformLocation;
    uChannels: WebGLUniformLocation;
    uChannelMode: WebGLUniformLocation;
  } | null = null;
  let rawInfo = $state<{
    width: number;
    height: number;
    channels: number;
    stats: fits.ChannelStats[];
    balance: number[];
  } | null>(null);

  let displayCanvas: HTMLCanvasElement;
  let ctx2d: CanvasRenderingContext2D | null = null;

  // ── GLSL shaders ──────────────────────────────────────────────────────────
  const VS = `#version 300 es
in vec2 aPos;
out vec2 vUv;
void main() {
  vUv = vec2(aPos.x * 0.5 + 0.5, 1.0 - (aPos.y * 0.5 + 0.5));
  gl_Position = vec4(aPos, 0.0, 1.0);
}`;

  // uChannelMode: 0=all, 1=R only, 2=G only, 3=B only (only when uChannels==3)
  const FS = `#version 300 es
precision highp float;
uniform sampler2D uTex;
uniform vec3  uGain;
uniform float uShadow;
uniform float uMidtone;
uniform bool  uLinear;
uniform int   uChannels;
uniform int   uChannelMode;
in  vec2  vUv;
out vec4  fragColor;

float mtf(float m, float x) {
  if (x <= 0.0) return 0.0;
  if (x >= 1.0) return 1.0;
  if (m <= 0.0) return 0.0;
  if (m >= 1.0) return 1.0;
  return (m - 1.0) * x / ((2.0 * m - 1.0) * x - m);
}
// Linked stretch (see lib/stretchPreview.ts): per-channel balance gain, then
// the same black point and midtone for every channel.
float applyStretch(float gain, float v) {
  float scale = 1.0 - uShadow;
  if (scale <= 0.0) return 0.0;
  float x = clamp((v * gain - uShadow) / scale, 0.0, 1.0);
  if (uLinear) return x;
  return clamp(mtf(uMidtone, x), 0.0, 1.0);
}
void main() {
  vec4 raw = texture(uTex, vUv);
  if (uChannels == 1 || uChannelMode == 1) {
    float v = applyStretch(uGain.x, raw.r);
    fragColor = vec4(v, v, v, 1.0);
  } else if (uChannelMode == 2) {
    float v = applyStretch(uGain.y, raw.g);
    fragColor = vec4(v, v, v, 1.0);
  } else if (uChannelMode == 3) {
    float v = applyStretch(uGain.z, raw.b);
    fragColor = vec4(v, v, v, 1.0);
  } else {
    fragColor = vec4(
      applyStretch(uGain.x, raw.r),
      applyStretch(uGain.y, raw.g),
      applyStretch(uGain.z, raw.b),
      1.0
    );
  }
}`;

  // ── 2D canvas display ─────────────────────────────────────────────────────

  function redraw2d() {
    if (!ctx2d || !glCanvas || !rawInfo) return;
    const cW = displayCanvas.width;
    const cH = displayCanvas.height;
    ctx2d.clearRect(0, 0, cW, cH);
    ctx2d.fillStyle = "#08090f";
    ctx2d.fillRect(0, 0, cW, cH);

    const cssScale = Math.min(cW / rawInfo.width, cH / rawInfo.height, 1.0);
    const dW = zoom * cssScale * rawInfo.width;
    const dH = zoom * cssScale * rawInfo.height;
    const destX = (cW - dW) / 2 + panX;
    const destY = (cH - dH) / 2 + panY;

    ctx2d.drawImage(glCanvas, destX, destY, dW, dH);
  }

  let _renderPending = false;
  function scheduleRender() {
    if (_renderPending) return;
    _renderPending = true;
    requestAnimationFrame(() => {
      _renderPending = false;
      renderGL();
    });
  }

  // ── Offscreen WebGL render ────────────────────────────────────────────────

  function renderGL(
    stats?: fits.ChannelStats[],
    enabled?: boolean,
    level?: number,
    chMode?: 0 | 1 | 2 | 3,
  ) {
    if (!gl || !program || !glTex || !glU || !rawInfo || !glCanvas || gl.isContextLost()) return;
    const on = enabled ?? (pp.stretchEnabled && !isProcessed);
    const st = computeStretch(
      stats ?? rawInfo.stats,
      rawInfo.balance,
      on ? (level ?? pp.stretchLevel) : 0,
    );
    gl.viewport(0, 0, glCanvas.width, glCanvas.height);
    gl.useProgram(program);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, glTex);
    gl.uniform1i(glU.uTex, 0);
    gl.uniform3fv(glU.uGain, st.gains);
    gl.uniform1f(glU.uShadow, st.shadows);
    gl.uniform1f(glU.uMidtone, st.midtone);
    gl.uniform1i(glU.uLinear, st.linear ? 1 : 0);
    gl.uniform1i(glU.uChannels, rawInfo.channels);
    gl.uniform1i(glU.uChannelMode, chMode ?? channelMode);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
    redraw2d();
  }

  // ── WebGL helpers ────────────────────────────────────────────────────────

  function setupGLContext(ctx: WebGL2RenderingContext): boolean {
    gl = ctx;
    const compile = (type: number, src: string): WebGLShader | null => {
      const s = gl!.createShader(type)!;
      gl!.shaderSource(s, src);
      gl!.compileShader(s);
      if (!gl!.getShaderParameter(s, gl!.COMPILE_STATUS)) {
        gl!.deleteShader(s);
        return null;
      }
      return s;
    };
    const vs = compile(gl.VERTEX_SHADER, VS);
    const fs = compile(gl.FRAGMENT_SHADER, FS);
    if (!vs || !fs) {
      previewError = "Shader compile failed";
      return false;
    }
    program = gl.createProgram()!;
    gl.attachShader(program, vs);
    gl.attachShader(program, fs);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      previewError = "Shader link failed: " + gl.getProgramInfoLog(program);
      return false;
    }
    gl.deleteShader(vs);
    gl.deleteShader(fs);
    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
    const aPos = gl.getAttribLocation(program, "aPos");
    gl.enableVertexAttribArray(aPos);
    gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);
    glTex = gl.createTexture();
    glU = {
      uTex: gl.getUniformLocation(program, "uTex")!,
      uGain: gl.getUniformLocation(program, "uGain")!,
      uShadow: gl.getUniformLocation(program, "uShadow")!,
      uMidtone: gl.getUniformLocation(program, "uMidtone")!,
      uLinear: gl.getUniformLocation(program, "uLinear")!,
      uChannels: gl.getUniformLocation(program, "uChannels")!,
      uChannelMode: gl.getUniformLocation(program, "uChannelMode")!,
    };
    return true;
  }

  function uploadTexture(f32: Float32Array, width: number, height: number, linear: boolean) {
    if (!gl || !glTex || gl.isContextLost()) return;
    const filter = linear ? gl.LINEAR : gl.NEAREST;
    gl.bindTexture(gl.TEXTURE_2D, glTex);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA32F, width, height, 0, gl.RGBA, gl.FLOAT, f32);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  }

  function createGLCanvas(width: number, height: number): boolean {
    // Reuse the existing context: resizing a canvas keeps its GL state, and creating a
    // fresh context per frame quickly exhausts the browser's context limit when culling.
    if (gl && program && glTex && glU && glCanvas && !gl.isContextLost()) {
      glCanvas.width = width;
      glCanvas.height = height;
      return true;
    }
    if (gl && !gl.isContextLost()) {
      if (program) gl.deleteProgram(program);
      if (glTex) gl.deleteTexture(glTex);
    }
    gl = null;
    program = null;
    glTex = null;
    glU = null;

    glCanvas = document.createElement("canvas");
    glCanvas.width = width;
    glCanvas.height = height;
    const ctx = glCanvas.getContext("webgl2", { preserveDrawingBuffer: true });
    if (!ctx) {
      previewError = "WebGL2 not supported";
      return false;
    }
    return setupGLContext(ctx);
  }

  function initDisplay(node: HTMLCanvasElement) {
    displayCanvas = node;
    node.width = viewportW || 800;
    node.height = viewportH || 600;
    ctx2d = node.getContext("2d");
    return {
      destroy() {
        ctx2d = null;
      },
    };
  }

  $effect(() => {
    const w = viewportW;
    const h = viewportH;
    if (!displayCanvas || w === 0 || h === 0) return;
    if (displayCanvas.width === w && displayCanvas.height === h) return;
    displayCanvas.width = w;
    displayCanvas.height = h;
    redraw2d();
  });

  // ── Load when the entry's path changes ───────────────────────────────────
  // Keyed on the path only, so metadata updates (reject, retype) don't reload pixels.

  const LOAD_DELAY_MS = 60;
  let loadTimer: ReturnType<typeof setTimeout> | undefined;

  $effect(() => {
    const path = entryPath;
    untrack(() => startLoad(path));
    return () => clearTimeout(loadTimer);
  });

  function startLoad(path: string) {
    clearTimeout(loadTimer);
    previewError = "";
    fitsHeader = null;
    rawInfo = null;
    hasImage = false;
    rasterDataUrl = "";
    histBins = null;
    annotations = [];
    showAnnotations = false;
    resetView();

    const id = ++previewReqId;

    if (isRasterFile(path)) {
      previewLoading = true;
      LoadRasterImage(path)
        .then((dataUrl: string) => {
          if (id !== previewReqId) return;
          rasterDataUrl = dataUrl;
          hasImage = true;
          previewLoading = false;
        })
        .catch((err: unknown) => {
          if (id !== previewReqId) return;
          previewError = err instanceof Error ? err.message : "Failed to load image";
          previewLoading = false;
        });
      return;
    }

    const cached = peekPreview(path);
    if (cached) {
      showPreview(cached, id);
      return;
    }

    previewLoading = true;
    // Short delay so holding an arrow key doesn't queue a backend render per row.
    loadTimer = setTimeout(() => {
      if (id !== previewReqId) return;
      loadPreview(path)
        .then((p) => {
          if (id !== previewReqId) return;
          showPreview(p, id);
        })
        .catch((err: unknown) => {
          if (id !== previewReqId) return;
          previewLoading = false;
          previewError = (err instanceof Error ? err.message : String(err)) || "Preview failed";
        });
    }, LOAD_DELAY_MS);
  }

  function showPreview(p: DecodedPreview, id: number) {
    previewLoading = false;
    fitsHeader = p.header;
    rawInfo = {
      width: p.width,
      height: p.height,
      channels: p.channels,
      stats: p.stats,
      balance: p.balance,
    };
    if (!createGLCanvas(p.width, p.height)) return;
    uploadTexture(p.pixels, p.width, p.height, false);
    renderGL();
    hasImage = true;

    if (p.hist) {
      histBins = p.hist;
    } else {
      setTimeout(() => {
        if (id !== previewReqId) return;
        p.hist = computeHistBins(p.pixels, p.channels);
        histBins = p.hist;
      }, 0);
    }
  }

  // Warm the cache for the likely-next frame once the current one is on screen.
  $effect(() => {
    const next = prefetch;
    if (!next || previewLoading || next === entryPath || isRasterFile(next)) return;
    const t = setTimeout(() => prefetchPreview(next), 120);
    return () => clearTimeout(t);
  });

  function applyStretch() {
    renderGL();
  }
  // The stretch prefs are shared (Blink changes them too): re-render on any change.
  $effect(() => {
    void pp.stretchEnabled;
    void pp.stretchLevel;
    scheduleRender();
  });
  function setStretch(l: number) {
    pp.stretchLevel = l;
    renderGL();
  }
  function setChannelMode(m: 0 | 1 | 2 | 3) {
    channelMode = m;
    renderGL();
  }

  // ── Histogram ────────────────────────────────────────────────────────────

  function computeHistBins(f32: Float32Array, channels: number): HistBins {
    const r = new Float32Array(256);
    const g = new Float32Array(256);
    const b = new Float32Array(256);
    for (let i = 0; i < f32.length; i += 4) {
      r[Math.min(255, Math.max(0, (f32[i] * 255) | 0))]++;
      g[Math.min(255, Math.max(0, (f32[i + 1] * 255) | 0))]++;
      b[Math.min(255, Math.max(0, (f32[i + 2] * 255) | 0))]++;
    }
    let peak = 1;
    for (let i = 1; i < 256; i++) {
      if (r[i] > peak) peak = r[i];
      if (channels > 1 && g[i] > peak) peak = g[i];
      if (channels > 1 && b[i] > peak) peak = b[i];
    }
    for (let i = 0; i < 256; i++) {
      r[i] /= peak;
      g[i] /= peak;
      b[i] /= peak;
    }
    return { r, g, b, channels };
  }

  $effect(() => {
    const hb = histBins;
    const hc = histCanvas;
    const se = pp.stretchEnabled;
    const sl = pp.stretchLevel;
    const ri = rawInfo;
    if (!showHistogram || !hb || !hc || !ri) return;
    const ctx2 = hc.getContext("2d");
    if (ctx2) drawHistogram(ctx2, hb, ri.stats, ri.balance, se, sl);
  });

  function drawHistogram(
    ctx2: CanvasRenderingContext2D,
    hb: HistBins,
    stats: fits.ChannelStats[],
    balance: number[],
    enabled: boolean,
    level: number,
  ) {
    const W = 200,
      H = 70;
    ctx2.clearRect(0, 0, W, H);
    ctx2.fillStyle = "rgba(8,9,15,0.82)";
    ctx2.fillRect(0, 0, W, H);

    const bw = W / 256;
    const layers: [Float32Array, string][] =
      hb.channels === 1
        ? [[hb.r, "rgba(160,160,160,0.75)"]]
        : [
            [hb.b, "rgba(80,140,255,0.5)"],
            [hb.g, "rgba(80,210,80,0.5)"],
            [hb.r, "rgba(255,90,90,0.5)"],
          ];

    for (const [data, color] of layers) {
      ctx2.fillStyle = color;
      for (let i = 1; i < 256; i++) {
        const h = data[i] * H;
        ctx2.fillRect(i * bw, H - h, bw + 0.5, h);
      }
    }

    if (stats.length > 0) {
      // Black point in the brightest channel's units (its balance gain is 1).
      const { shadows } = computeStretch(stats, balance, enabled ? level : 0);
      if (shadows > 0 && shadows < 1) {
        const sx = shadows * W;
        ctx2.strokeStyle = "rgba(255,200,50,0.85)";
        ctx2.lineWidth = 1;
        ctx2.setLineDash([2, 2]);
        ctx2.beginPath();
        ctx2.moveTo(sx, 0);
        ctx2.lineTo(sx, H);
        ctx2.stroke();
        ctx2.setLineDash([]);
      }
    }

    ctx2.strokeStyle = "rgba(255,255,255,0.12)";
    ctx2.lineWidth = 0.5;
    ctx2.strokeRect(0, 0, W, H);
  }

  // ── Annotations ──────────────────────────────────────────────────────────

  let canAnnotate = $derived(
    !!(qualityFrame?.wcsSolved || (fitsHeader?.ra && fitsHeader.pixelScale > 0)),
  );

  // Without a plate solve, positions come from the FITS header's pointing, which
  // can be off by a fraction of the field and may carry no usable rotation.
  let annotationsApprox = $derived(!qualityFrame?.wcsSolved);

  // Fetched as soon as the frame is shown (cheap projection math, no file I/O) so the
  // Labels button only appears when a catalog object actually falls inside the field.
  // Debounced so holding an arrow key doesn't call the backend for every row.
  const ANNOTATION_DELAY_MS = 150;

  $effect(() => {
    if (!canAnnotate || !rawInfo) return;
    const solved = !!qualityFrame?.wcsSolved;
    const ra = solved ? qualityFrame!.ra : (fitsHeader?.ra ?? 0);
    const dec = solved ? qualityFrame!.dec : (fitsHeader?.dec ?? 0);
    const scale = solved ? qualityFrame!.pixelScale : (fitsHeader?.pixelScale ?? 0);
    const rot = solved ? qualityFrame!.rotation : (fitsHeader?.rotation ?? 0);
    const { width, height } = rawInfo;
    // Cleanup runs when the frame or its WCS changes, so a late reply can't
    // land on the wrong frame.
    let stale = false;
    const t = setTimeout(() => {
      GetAnnotations(ra, dec, scale, rot, width, height)
        .then((res) => {
          if (!stale) annotations = res ?? [];
        })
        .catch(() => {
          if (!stale) annotations = [];
        });
    }, ANNOTATION_DELAY_MS);
    return () => {
      stale = true;
      clearTimeout(t);
    };
  });

  function imgToViewport(imgX: number, imgY: number): { x: number; y: number } {
    if (!rawInfo || viewportW === 0 || viewportH === 0) return { x: -9999, y: -9999 };
    const cssScale = Math.min(viewportW / rawInfo.width, viewportH / rawInfo.height, 1);
    const cx = viewportW / 2;
    const cy = viewportH / 2;
    const dx = (imgX - rawInfo.width / 2) * cssScale;
    const dy = (imgY - rawInfo.height / 2) * cssScale;
    return {
      x: cx + dx * zoom + panX,
      y: cy + dy * zoom + panY,
    };
  }

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.15 : 0.87;
    const newZoom = Math.max(0.1, Math.min(20, zoom * factor));
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const mx = e.clientX - rect.left - rect.width / 2;
    const my = e.clientY - rect.top - rect.height / 2;
    panX = mx - ((mx - panX) * newZoom) / zoom;
    panY = my - ((my - panY) * newZoom) / zoom;
    zoom = newZoom;
    scheduleRender();
  }

  function onPanStart(e: MouseEvent) {
    if (e.button !== 0) return;
    isPanning = true;
    panStartX = e.clientX - panX;
    panStartY = e.clientY - panY;
  }
  function onPanMove(e: MouseEvent) {
    if (!isPanning) return;
    panX = e.clientX - panStartX;
    panY = e.clientY - panStartY;
    scheduleRender();
  }
  function onPanEnd() {
    isPanning = false;
  }
</script>

<div class="preview-pane">
  <div class="preview-titlebar">
    {#if onprev || onnext}
      <div class="pv-group">
        <button
          class="pv-btn pv-icon"
          onclick={onprev}
          disabled={!onprev}
          aria-label="Previous frame"
          title="Previous frame (↑ / k)">◀</button
        >
        <button
          class="pv-btn pv-icon"
          onclick={onnext}
          disabled={!onnext}
          aria-label="Next frame"
          title="Next frame (↓ / j)">▶</button
        >
      </div>
    {/if}
    <span class="preview-filename" title={entry.path}>{entry.name}</span>
    {#if isRejected}<span class="pv-rejected">Rejected</span>{/if}
    <div class="pv-group">
      {#if isRejected && onrestore}
        <button class="pv-btn" onclick={onrestore} title="Restore this frame (u)">Restore</button>
      {:else if !isRejected && onreject}
        <button class="pv-btn pv-reject" onclick={onreject} title="Reject this frame (x)"
          >Reject</button
        >
      {/if}
      {#if ondelete}
        <button
          class="pv-btn pv-danger"
          onclick={ondelete}
          title="Delete this file from disk (Delete)">Delete…</button
        >
      {/if}
      {#if onblink}
        <button class="pv-btn" onclick={onblink} title={blinkTitle}>▶ Blink</button>
      {/if}
      {#if onreveal}
        <button class="pv-btn" onclick={onreveal} title="Show in the system file manager"
          >Show in folder</button
        >
      {/if}
    </div>
    <button
      class="pv-close"
      onclick={onclose}
      aria-label="Close preview"
      title="Close preview (Esc)">✕</button
    >
  </div>

  <div class="preview-toolbar">
    <div class="pv-group">
      <button class="tool-btn" onclick={resetView} title="Fit to window (double-click image)"
        >Fit</button
      >
      <span class="zoom-label">{Math.round(zoom * 100)}%</span>
    </div>

    <div class="pv-group pv-view">
      {#if !isProcessed && !isRaster}
        <div class="seg-group" role="group" aria-label="Autostretch">
          <button
            class="tool-btn"
            class:active={pp.stretchEnabled}
            onclick={() => {
              pp.stretchEnabled = !pp.stretchEnabled;
              applyStretch();
            }}
            title="Toggle autostretch">Stretch</button
          >
          {#if pp.stretchEnabled}
            <button
              class="tool-btn preset"
              class:active={pp.stretchLevel === 1}
              onclick={() => setStretch(1)}>Gentle</button
            >
            <button
              class="tool-btn preset"
              class:active={pp.stretchLevel === 2}
              onclick={() => setStretch(2)}>Normal</button
            >
            <button
              class="tool-btn preset"
              class:active={pp.stretchLevel === 3}
              onclick={() => setStretch(3)}>Strong</button
            >
          {/if}
        </div>
      {/if}

      {#if rawInfo && !isRaster && rawInfo.channels === 3}
        <div class="seg-group" role="group" aria-label="Channel">
          <button
            class="tool-btn ch-btn"
            class:active={channelMode === 0}
            onclick={() => setChannelMode(0)}>RGB</button
          >
          <button
            class="tool-btn ch-btn ch-r"
            class:active={channelMode === 1}
            onclick={() => setChannelMode(1)}>R</button
          >
          <button
            class="tool-btn ch-btn ch-g"
            class:active={channelMode === 2}
            onclick={() => setChannelMode(2)}>G</button
          >
          <button
            class="tool-btn ch-btn ch-b"
            class:active={channelMode === 3}
            onclick={() => setChannelMode(3)}>B</button
          >
        </div>
      {/if}

      {#if rawInfo && !isRaster}
        <button
          class="tool-btn"
          class:active={showHistogram}
          onclick={() => (showHistogram = !showHistogram)}
          title="Histogram overlay">Hist</button
        >
      {/if}

      {#if annotations.length > 0}
        <button
          class="tool-btn"
          class:active={showAnnotations}
          onclick={() => (showAnnotations = !showAnnotations)}
          title={annotationsApprox
            ? "Star / DSO labels — approximate: this frame isn't plate-solved, so positions come from the FITS header's pointing"
            : "Star / DSO labels (plate-solved)"}>✦ Labels{annotationsApprox ? " ≈" : ""}</button
        >
      {/if}
    </div>
  </div>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="image-viewport"
    class:panning={isPanning && !isRaster}
    bind:this={viewportEl}
    onwheel={isRaster ? undefined : onWheel}
    onmousedown={isRaster ? undefined : onPanStart}
    onmousemove={isRaster ? undefined : onPanMove}
    onmouseup={isRaster ? undefined : onPanEnd}
    onmouseleave={isRaster ? undefined : onPanEnd}
    ondblclick={isRaster ? undefined : resetView}
  >
    {#if isRaster}
      <img
        src={rasterDataUrl}
        alt={entry.name}
        class="raster-img"
        class:visible={hasImage}
        draggable="false"
      />
    {:else}
      <canvas
        use:initDisplay
        class="display-canvas"
        class:visible={hasImage && !previewLoading}
        draggable="false"
      ></canvas>
    {/if}

    <!-- Annotation overlay — absolute, viewport-space coordinates computed by imgToViewport() -->
    {#if showAnnotations && annotations.length > 0 && rawInfo && viewportW > 0}
      <svg class="annotation-svg" width={viewportW} height={viewportH}>
        {#if annotationsApprox}
          <text x="8" y={viewportH - 8} font-size="12" fill="rgba(255,200,50,0.55)"
            >Approximate positions — frame not plate-solved</text
          >
        {/if}
        {#each annotations as ann (`${ann.label}${ann.x}${ann.y}`)}
          {@const vp = imgToViewport(ann.x, ann.y)}
          {#if ann.type === "star"}
            <circle
              cx={vp.x}
              cy={vp.y}
              r="7"
              fill="none"
              stroke="rgba(136,196,255,0.75)"
              stroke-width="0.9"
            />
            <text
              x={vp.x}
              y={vp.y + 16}
              font-size="12"
              fill="rgba(136,196,255,0.95)"
              text-anchor="middle"
              class="ann-lbl">{ann.label}</text
            >
          {:else}
            <line
              x1={vp.x - 9}
              y1={vp.y}
              x2={vp.x + 9}
              y2={vp.y}
              stroke="rgba(255,204,68,0.8)"
              stroke-width="0.9"
            />
            <line
              x1={vp.x}
              y1={vp.y - 9}
              x2={vp.x}
              y2={vp.y + 9}
              stroke="rgba(255,204,68,0.8)"
              stroke-width="0.9"
            />
            <text
              x={vp.x}
              y={vp.y + 17}
              font-size="12"
              fill="rgba(255,204,68,1)"
              text-anchor="middle"
              class="ann-lbl">{ann.label}</text
            >
          {/if}
        {/each}
      </svg>
    {/if}

    <!-- Histogram overlay — fixed corner, not transformed with image -->
    {#if showHistogram && rawInfo}
      <canvas bind:this={histCanvas} class="hist-canvas" width={200} height={70}></canvas>
    {/if}

    {#if previewLoading}
      <div class="preview-overlay">
        <span class="spinner">◌</span> Generating preview…
      </div>
    {:else if previewError}
      <div class="preview-error">{previewError}</div>
    {/if}
  </div>

  {#if fitsHeader || (isRaster && qualityFrame)}
    <div class="preview-meta">
      <div class="meta-section">
        <button class="meta-section-hdr" onclick={() => (pp.basicCollapsed = !pp.basicCollapsed)}>
          <span>Basic</span>
          <span class="meta-caret">{pp.basicCollapsed ? "›" : "⌄"}</span>
        </button>
        {#if !pp.basicCollapsed}
          {#if fitsHeader}
            {#each basicRows(fitsHeader) as row (row.key)}
              <div class="meta-row">
                <span class="meta-key">{row.key}</span>
                <span class="meta-val">{row.val}</span>
              </div>
            {/each}
          {:else if qualityFrame}
            {#if qualityFrame.object}
              <div class="meta-row">
                <span class="meta-key">Object</span>
                <span class="meta-val">{qualityFrame.object}</span>
              </div>
            {/if}
            <div class="meta-row">
              <span class="meta-key">Type</span>
              <span class="meta-val">{qualityFrame.frameType}</span>
            </div>
            {#if qualityFrame.telescope}
              <div class="meta-row">
                <span class="meta-key">Telescope</span>
                <span class="meta-val">{qualityFrame.telescope}</span>
              </div>
            {/if}
            {#if qualityFrame.instrument}
              <div class="meta-row">
                <span class="meta-key">Camera</span>
                <span class="meta-val">{qualityFrame.instrument}</span>
              </div>
            {/if}
          {/if}
        {/if}
      </div>
      {#if qualityFrame || fitsHeader?.ra}
        <div class="meta-section">
          <button class="meta-section-hdr" onclick={() => (coordsCollapsed = !coordsCollapsed)}>
            <span>Coordinates</span>
            {#if qualityFrame?.wcsSolved}<span class="stats-badge">✦</span>{/if}
            <span class="meta-caret">{coordsCollapsed ? "›" : "⌄"}</span>
          </button>
          {#if !coordsCollapsed}
            {#if qualityFrame?.wcsSolved}
              <div class="meta-row">
                <span class="meta-key">RA</span><span class="meta-val"
                  >{formatRA(qualityFrame.ra)}</span
                >
              </div>
              <div class="meta-row">
                <span class="meta-key">Dec</span><span class="meta-val"
                  >{formatDec(qualityFrame.dec)}</span
                >
              </div>
              {#if qualityFrame.pixelScale}
                <div class="meta-row">
                  <span class="meta-key">Scale</span><span class="meta-val"
                    >{qualityFrame.pixelScale.toFixed(2)} "/px</span
                  >
                </div>
              {/if}
              {#if qualityFrame.rotation}
                <div class="meta-row">
                  <span class="meta-key">Rotation</span><span class="meta-val"
                    >{qualityFrame.rotation.toFixed(1)}°</span
                  >
                </div>
              {/if}
            {:else if fitsHeader?.ra}
              <div class="meta-row">
                <span class="meta-key">RA</span><span class="meta-val"
                  >{formatRA(fitsHeader.ra)}</span
                >
              </div>
              <div class="meta-row">
                <span class="meta-key">Dec</span><span class="meta-val"
                  >{formatDec(fitsHeader.dec)}</span
                >
              </div>
            {:else if qualityFrame}
              <div class="meta-row stats-hint">
                <span class="meta-val"
                  >Not plate solved. Use <strong>✦ Analyze</strong> in the Library View.</span
                >
              </div>
            {/if}
          {/if}
        </div>
      {/if}
      {#if fitsHeader}
        <div class="meta-section">
          <button
            class="meta-section-hdr"
            onclick={() => (pp.advancedCollapsed = !pp.advancedCollapsed)}
          >
            <span>Advanced</span>
            <span class="meta-caret">{pp.advancedCollapsed ? "›" : "⌄"}</span>
          </button>
          {#if !pp.advancedCollapsed}
            {#each advancedRows(fitsHeader) as row (row.key)}
              <div class="meta-row">
                <span class="meta-key">{row.key}</span>
                <span class="meta-val">{row.val}</span>
              </div>
            {/each}
          {/if}
        </div>
      {/if}
      {#if qualityFrame}
        <div class="meta-section">
          <button class="meta-section-hdr" onclick={() => (statsCollapsed = !statsCollapsed)}>
            <span>Statistics</span>
            {#if qualityFrame.qualityAnalyzed}<span class="stats-badge">✦</span>{/if}
            <span class="meta-caret">{statsCollapsed ? "›" : "⌄"}</span>
          </button>
          {#if !statsCollapsed}
            {#if qualityFrame.qualityAnalyzed}
              <div class="meta-row">
                <span class="meta-key">Stars</span><span class="meta-val"
                  >{qualityFrame.starCount}</span
                >
              </div>
              <div class="meta-row">
                <span class="meta-key">FWHM</span><span class="meta-val"
                  >{qualityFrame.fwhm.toFixed(2)} {qualityFrame.fwhmUnit || "px"}</span
                >
              </div>
              {#if qualityFrame.background}
                <div class="meta-row">
                  <span class="meta-key">Background</span><span class="meta-val"
                    >{qualityFrame.background.toFixed(1)} ADU</span
                  >
                </div>
              {/if}
              {#if qualityFrame.noise}
                <div class="meta-row">
                  <span class="meta-key">Noise</span><span class="meta-val"
                    >{qualityFrame.noise.toFixed(2)} ADU</span
                  >
                </div>
              {/if}
              {#if qualityFrame.snr}
                <div class="meta-row">
                  <span class="meta-key">SNR</span><span class="meta-val"
                    >{qualityFrame.snr.toFixed(1)}</span
                  >
                </div>
              {/if}
            {:else}
              <div class="meta-row stats-hint">
                <span class="meta-val"
                  >Not analyzed. Use <strong>✦ Analyze</strong> in the Library View.</span
                >
              </div>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .preview-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
  }

  .preview-titlebar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 5px 8px 5px 10px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    min-width: 0;
  }

  .preview-filename {
    font-size: var(--fs-sm);
    font-family: "Consolas", "Fira Code", monospace;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }

  .pv-rejected {
    font-size: var(--fs-xs);
    font-weight: 600;
    color: var(--danger);
    border: 1px solid var(--danger);
    border-radius: 3px;
    padding: 0 6px;
    flex-shrink: 0;
  }

  .pv-group {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }

  .pv-btn {
    height: 26px;
    padding: 0 10px;
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: var(--fs-xs);
    white-space: nowrap;
    cursor: pointer;
    transition:
      color 0.15s,
      border-color 0.15s,
      background 0.15s;
  }
  .pv-btn:hover:not(:disabled) {
    color: var(--accent);
    border-color: var(--accent);
  }
  .pv-btn:disabled {
    color: var(--text-dim);
    cursor: default;
  }
  .pv-icon {
    width: 28px;
    padding: 0;
  }
  .pv-reject:hover:not(:disabled),
  .pv-danger:hover:not(:disabled) {
    color: var(--danger);
    border-color: var(--danger);
  }

  .pv-close {
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    color: var(--text-secondary);
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: var(--fs-md);
    cursor: pointer;
    transition:
      color 0.15s,
      border-color 0.15s;
  }
  .pv-close:hover {
    color: var(--text-primary);
    border-color: var(--text-secondary);
    background: var(--bg-row-hover);
  }

  .preview-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 4px 10px;
    padding: 4px 10px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .pv-view {
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
    flex-shrink: 1;
  }

  .zoom-label {
    font-size: var(--fs-xs);
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
    min-width: 36px;
  }

  .seg-group {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .ch-btn {
    min-width: 24px;
    padding: 2px 5px;
    font-weight: 700;
  }
  .ch-r.active,
  .ch-r.active:hover {
    background: #ff6060;
    border-color: #ff6060;
  }
  .ch-g.active,
  .ch-g.active:hover {
    background: #50d050;
    border-color: #50d050;
  }
  .ch-b.active,
  .ch-b.active:hover {
    background: #6098ff;
    border-color: #6098ff;
  }

  /* ── Image viewport ──────────────────────────────────────────────────── */
  .image-viewport {
    flex: 1;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: grab;
    position: relative;
    background: #08090f;
  }
  .image-viewport.panning {
    cursor: grabbing;
  }

  .display-canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    display: block;
    user-select: none;
    pointer-events: none;
    visibility: hidden;
  }
  .display-canvas.visible {
    visibility: visible;
  }

  .raster-img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    user-select: none;
    visibility: hidden;
  }
  .raster-img.visible {
    visibility: visible;
  }

  /* ── Annotation SVG (absolute, viewport-space) ───────────────────────── */
  .annotation-svg {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: none;
    overflow: visible;
  }

  :global(.ann-lbl) {
    font-family: "Consolas", "Fira Code", monospace;
    paint-order: stroke fill;
    stroke: rgba(0, 0, 0, 0.7);
    stroke-width: 2.5px;
  }

  /* ── Histogram ───────────────────────────────────────────────────────── */
  .hist-canvas {
    position: absolute;
    bottom: 10px;
    right: 10px;
    border-radius: 4px;
    pointer-events: none;
    z-index: 5;
  }

  /* ── Loading / error overlays ────────────────────────────────────────── */
  .preview-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);
    font-size: var(--fs-md);
    gap: 8px;
    pointer-events: none;
  }
  .spinner {
    display: inline-block;
    animation: spin 1.2s linear infinite;
    color: var(--accent);
    font-size: 1.2rem;
  }
  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }
  .preview-error {
    position: absolute;
    color: var(--danger);
    font-size: var(--fs-sm);
    background: var(--bg-panel);
    border: 1px solid var(--danger);
    border-radius: 4px;
    padding: 10px 14px;
    max-width: 80%;
  }

  /* ── FITS metadata ────────────────────────────────────────────────────── */
  .preview-meta {
    flex-shrink: 0;
    padding: 0 14px 10px;
    overflow-y: auto;
    max-height: 220px;
    border-top: 1px solid var(--border);
  }
  .preview-meta::-webkit-scrollbar {
    width: 4px;
  }
  .preview-meta::-webkit-scrollbar-thumb {
    background: var(--border-accent);
    border-radius: 2px;
  }

  .meta-section {
    border-bottom: 1px solid var(--border);
  }
  .meta-section-hdr {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 5px 0 3px;
    color: var(--text-secondary);
    font-size: var(--fs-xs);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .meta-section-hdr:hover {
    color: var(--text-primary);
  }
  .meta-caret {
    font-size: var(--fs-sm);
  }

  .meta-row {
    display: flex;
    justify-content: space-between;
    padding: 2px 0;
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
  }
  .meta-key {
    color: var(--text-secondary);
    width: 80px;
    flex-shrink: 0;
  }
  .meta-val {
    color: var(--text-primary);
    text-align: right;
    font-variant-numeric: tabular-nums;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .stats-badge {
    font-size: var(--fs-xs);
    color: var(--accent);
    margin-left: 4px;
    margin-right: auto;
  }
  .stats-hint {
    font-style: italic;
  }
  .stats-hint .meta-val {
    text-align: left;
    white-space: normal;
    font-size: var(--fs-xs);
    color: var(--text-secondary);
  }
</style>
