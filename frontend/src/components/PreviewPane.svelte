<script lang="ts">
  import { onMount } from "svelte";
  import { untrack } from "svelte";
  import type { app } from "../../wailsjs/go/models";
  import { GeneratePreviewRaw, GeneratePreviewRawSized, ReadFITSHeader, GetAnnotations } from "../../wailsjs/go/app/App.js";
  import { basicRows, advancedRows, formatRA, formatDec } from "../lib/utils";

  interface Props {
    entry: app.EnrichedFileEntry;
    stretchEnabled: boolean;
    stretchLevel: number;
    basicCollapsed: boolean;
    advancedCollapsed: boolean;
    qualityFrame?: app.LibraryFrame | null;
    onclose: () => void;
  }

  let {
    entry,
    stretchEnabled = $bindable(),
    stretchLevel = $bindable(),
    basicCollapsed = $bindable(),
    advancedCollapsed = $bindable(),
    qualityFrame = null,
    onclose,
  }: Props = $props();

  let isProcessed = $derived(qualityFrame?.frameType === 'processed');

  let statsCollapsed = $state(false);
  let coordsCollapsed = $state(false);

  // ── Channel display mode (0=all, 1=R, 2=G, 3=B) ───────────────────────────
  let channelMode = $state<0 | 1 | 2 | 3>(0);

  // ── Histogram overlay ─────────────────────────────────────────────────────
  let showHistogram = $state(false);
  let histCanvas = $state<HTMLCanvasElement | null>(null);
  interface HistBins { r: Float32Array; g: Float32Array; b: Float32Array; channels: number; }
  let histBins = $state<HistBins | null>(null);

  // ── Annotation overlay ────────────────────────────────────────────────────
  let showAnnotations = $state(false);
  let annotations = $state<app.Annotation[]>([]);
  let annotationsLoading = $state(false);

  // ── Viewport size tracking (for annotation coordinate projection) ─────────
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

  // ── Preview load state ───────────────────────────────────────────────────
  let previewLoading = $state(false);
  let previewError = $state("");
  let fitsHeader = $state<app.FITSHeader | null>(null);
  let previewReqId = 0;
  let hasImage = $state(false);

  // ── Dynamic resolution ────────────────────────────────────────────────────
  // Tracks the maxSize used for the currently loaded texture (768 = initial).
  let loadedMaxSize = $state(768);
  let zoomReqId = 0;
  let upgradeTimer: ReturnType<typeof setTimeout> | null = null;

  // ── Zoom / pan ────────────────────────────────────────────────────────────
  let zoom = $state(1);
  let panX = $state(0);
  let panY = $state(0);
  let isPanning = $state(false);
  let panStartX = 0;
  let panStartY = 0;

  function resetView() { zoom = 1; panX = 0; panY = 0; }

  // ── WebGL state ───────────────────────────────────────────────────────────
  let canvas: HTMLCanvasElement;
  let gl: WebGL2RenderingContext | null = null;
  let program: WebGLProgram | null = null;
  let glTex: WebGLTexture | null = null;
  let glU: {
    uTex: WebGLUniformLocation;
    uShadows: WebGLUniformLocation;
    uMidtones: WebGLUniformLocation;
    uLinear: WebGLUniformLocation;
    uChannels: WebGLUniformLocation;
    uChannelMode: WebGLUniformLocation;
  } | null = null;
  let rawInfo = $state<{ width: number; height: number; channels: number; stats: app.ChannelStats[]; } | null>(null);

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
uniform vec3  uShadows;
uniform vec3  uMidtones;
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
float applyStretch(float shadow, float midtone, float v) {
  float scale = 1.0 - shadow;
  if (scale <= 0.0) return 0.0;
  float x = clamp((v - shadow) / scale, 0.0, 1.0);
  if (uLinear) return x;
  return clamp(mtf(midtone, x), 0.0, 1.0);
}
void main() {
  vec4 raw = texture(uTex, vUv);
  if (uChannels == 1) {
    float v = applyStretch(uShadows.x, uMidtones.x, raw.r);
    fragColor = vec4(v, v, v, 1.0);
  } else if (uChannelMode == 1) {
    float v = applyStretch(uShadows.x, uMidtones.x, raw.r);
    fragColor = vec4(v, v, v, 1.0);
  } else if (uChannelMode == 2) {
    float v = applyStretch(uShadows.y, uMidtones.y, raw.g);
    fragColor = vec4(v, v, v, 1.0);
  } else if (uChannelMode == 3) {
    float v = applyStretch(uShadows.z, uMidtones.z, raw.b);
    fragColor = vec4(v, v, v, 1.0);
  } else {
    float r = applyStretch(uShadows.x, uMidtones.x, raw.r);
    float g = applyStretch(uShadows.y, uMidtones.y, raw.g);
    float b = applyStretch(uShadows.z, uMidtones.z, raw.b);
    fragColor = vec4(r, g, b, 1.0);
  }
}`;

  // ── Stretch math ─────────────────────────────────────────────────────────

  function mtfMidtone(target: number, x: number): number {
    if (x === 0) return 0;
    const d = x * (1 - 2 * target) + target;
    if (d === 0) return 0;
    return (x * (1 - target)) / d;
  }

  interface StretchUniforms { shadows: number; midtone: number; linear: boolean; }

  function computeUniforms(stats: app.ChannelStats[], enabled: boolean, level: number): StretchUniforms[] {
    const presets = [
      { shadowsFactor: -1.25, targetBG: 0.1  },
      { shadowsFactor: -2.8,  targetBG: 0.25 },
      { shadowsFactor: -4.0,  targetBG: 0.4  },
    ];
    return stats.map((s) => {
      const { median: med, sigma: sig } = s;
      if (!enabled) return { shadows: Math.max(0, med - 2.8 * sig), midtone: 0.5, linear: true };
      const preset = presets[Math.max(0, Math.min(level - 1, presets.length - 1))];
      const shadows = Math.max(0, med + preset.shadowsFactor * sig);
      const scale = 1.0 - shadows;
      if (scale <= 0) return { shadows: 0, midtone: 0.25, linear: false };
      const newMedian = Math.max(0, med - shadows) / scale;
      const midtone = mtfMidtone(preset.targetBG, newMedian);
      return { shadows, midtone, linear: false };
    });
  }

  // ── WebGL render ──────────────────────────────────────────────────────────

  function renderGL() {
    if (!gl || !program || !glTex || !glU || !rawInfo || gl.isContextLost()) return;
    const uniforms = computeUniforms(rawInfo.stats, stretchEnabled && !isProcessed, stretchLevel);
    const u0 = uniforms[0] ?? { shadows: 0, midtone: 0.5, linear: true };
    const u1 = uniforms[1] ?? u0;
    const u2 = uniforms[2] ?? u0;
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.useProgram(program);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, glTex);
    gl.uniform1i(glU.uTex, 0);
    gl.uniform3fv(glU.uShadows,  [u0.shadows, u1.shadows, u2.shadows]);
    gl.uniform3fv(glU.uMidtones, [u0.midtone, u1.midtone, u2.midtone]);
    gl.uniform1i(glU.uLinear, u0.linear ? 1 : 0);
    gl.uniform1i(glU.uChannels, rawInfo.channels);
    gl.uniform1i(glU.uChannelMode, channelMode);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
  }

  function renderGLWith(stats: app.ChannelStats[], enabled: boolean, level: number, chMode: 0|1|2|3 = 0) {
    if (!gl || !program || !glTex || !glU || !rawInfo || gl.isContextLost()) return;
    const uniforms = computeUniforms(stats, enabled, level);
    const u0 = uniforms[0] ?? { shadows: 0, midtone: 0.5, linear: true };
    const u1 = uniforms[1] ?? u0;
    const u2 = uniforms[2] ?? u0;
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.useProgram(program);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, glTex);
    gl.uniform1i(glU.uTex, 0);
    gl.uniform3fv(glU.uShadows,  [u0.shadows, u1.shadows, u2.shadows]);
    gl.uniform3fv(glU.uMidtones, [u0.midtone, u1.midtone, u2.midtone]);
    gl.uniform1i(glU.uLinear, u0.linear ? 1 : 0);
    gl.uniform1i(glU.uChannels, rawInfo.channels);
    gl.uniform1i(glU.uChannelMode, chMode);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
  }

  // ── WebGL init ────────────────────────────────────────────────────────────

  onMount(() => {
    const ctx = canvas.getContext("webgl2");
    if (!ctx) { previewError = "WebGL2 not supported"; return; }
    gl = ctx;

    const compileShader = (type: number, src: string): WebGLShader | null => {
      const s = gl!.createShader(type)!;
      gl!.shaderSource(s, src);
      gl!.compileShader(s);
      if (!gl!.getShaderParameter(s, gl!.COMPILE_STATUS)) {
        console.error("Shader error:", gl!.getShaderInfoLog(s));
        gl!.deleteShader(s);
        return null;
      }
      return s;
    };

    const vs = compileShader(gl.VERTEX_SHADER, VS);
    const fs = compileShader(gl.FRAGMENT_SHADER, FS);
    if (!vs || !fs) { previewError = "Shader compile failed"; return; }

    program = gl.createProgram()!;
    gl.attachShader(program, vs);
    gl.attachShader(program, fs);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      previewError = "Shader link failed: " + gl.getProgramInfoLog(program);
      return;
    }
    gl.deleteShader(vs);
    gl.deleteShader(fs);

    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1, 1,-1, -1,1, 1,1]), gl.STATIC_DRAW);
    const aPos = gl.getAttribLocation(program, "aPos");
    gl.enableVertexAttribArray(aPos);
    gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

    glTex = gl.createTexture();
    glU = {
      uTex:         gl.getUniformLocation(program, "uTex")!,
      uShadows:     gl.getUniformLocation(program, "uShadows")!,
      uMidtones:    gl.getUniformLocation(program, "uMidtones")!,
      uLinear:      gl.getUniformLocation(program, "uLinear")!,
      uChannels:    gl.getUniformLocation(program, "uChannels")!,
      uChannelMode: gl.getUniformLocation(program, "uChannelMode")!,
    };
  });

  // ── Load when entry changes ───────────────────────────────────────────────

  $effect(() => {
    const e = entry;
    previewLoading = true;
    previewError   = "";
    fitsHeader     = null;
    rawInfo        = null;
    hasImage       = false;
    histBins       = null;
    annotations    = [];
    showAnnotations = false;
    loadedMaxSize  = 768;
    if (upgradeTimer) { clearTimeout(upgradeTimer); upgradeTimer = null; }
    resetView();

    const id = ++previewReqId;
    const se = untrack(() => stretchEnabled);
    const sl = untrack(() => stretchLevel);

    Promise.allSettled([ReadFITSHeader(e.path), GeneratePreviewRaw(e.path)]).then(
      ([hdrResult, rawResult]) => {
        if (id !== previewReqId) return;
        previewLoading = false;

        if (hdrResult.status === "fulfilled") fitsHeader = hdrResult.value;

        if (rawResult.status === "fulfilled") {
          const result = rawResult.value;

          const bin = atob(result.data);
          const u8 = new Uint8Array(bin.length);
          for (let i = 0; i < bin.length; i++) u8[i] = bin.charCodeAt(i);
          const f32 = new Float32Array(u8.buffer);

          rawInfo = { width: result.width, height: result.height, channels: result.channels, stats: result.stats };

          canvas.width  = result.width;
          canvas.height = result.height;

          if (gl && glTex) {
            gl.bindTexture(gl.TEXTURE_2D, glTex);
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA32F, result.width, result.height, 0, gl.RGBA, gl.FLOAT, f32);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
          }

          const qf = untrack(() => qualityFrame);
          renderGLWith(rawInfo.stats, se && qf?.frameType !== 'processed', sl, 0);
          hasImage = true;

          // Compute histogram off the main render path
          setTimeout(() => { histBins = computeHistBins(f32, result.channels); }, 0);
        } else {
          previewError = (rawResult as PromiseRejectedResult).reason?.toString() ?? "Preview failed";
        }
      },
    );
  });

  // ── Stretch / channel controls ────────────────────────────────────────────

  function applyStretch()         { renderGL(); }
  function setStretch(l: number)  { stretchLevel = l; renderGL(); }
  function setChannelMode(m: 0|1|2|3) { channelMode = m; renderGL(); }

  // ── Histogram ─────────────────────────────────────────────────────────────

  function computeHistBins(f32: Float32Array, channels: number): HistBins {
    const r = new Float32Array(256);
    const g = new Float32Array(256);
    const b = new Float32Array(256);
    for (let i = 0; i < f32.length; i += 4) {
      r[Math.min(255, Math.max(0, (f32[i]   * 255) | 0))]++;
      g[Math.min(255, Math.max(0, (f32[i+1] * 255) | 0))]++;
      b[Math.min(255, Math.max(0, (f32[i+2] * 255) | 0))]++;
    }
    // Normalize excluding bin 0 (black sky background dominates)
    let peak = 1;
    for (let i = 1; i < 256; i++) {
      if (r[i] > peak) peak = r[i];
      if (channels > 1 && g[i] > peak) peak = g[i];
      if (channels > 1 && b[i] > peak) peak = b[i];
    }
    for (let i = 0; i < 256; i++) { r[i] /= peak; g[i] /= peak; b[i] /= peak; }
    return { r, g, b, channels };
  }

  // Redraw histogram whenever the canvas is mounted or data/stretch changes.
  $effect(() => {
    const hb  = histBins;
    const hc  = histCanvas;
    const se  = stretchEnabled;
    const sl  = stretchLevel;
    const ri  = rawInfo;
    if (!showHistogram || !hb || !hc || !ri) return;
    const ctx2 = hc.getContext("2d");
    if (ctx2) drawHistogram(ctx2, hb, ri.stats, se, sl);
  });

  function drawHistogram(ctx2: CanvasRenderingContext2D, hb: HistBins, stats: app.ChannelStats[], enabled: boolean, level: number) {
    const W = 200, H = 70;
    ctx2.clearRect(0, 0, W, H);
    ctx2.fillStyle = "rgba(8,9,15,0.82)";
    ctx2.fillRect(0, 0, W, H);

    const bw = W / 256;
    const layers: [Float32Array, string][] =
      hb.channels === 1
        ? [[hb.r, "rgba(160,160,160,0.75)"]]
        : [[hb.b, "rgba(80,140,255,0.5)"], [hb.g, "rgba(80,210,80,0.5)"], [hb.r, "rgba(255,90,90,0.5)"]];

    for (const [data, color] of layers) {
      ctx2.fillStyle = color;
      for (let i = 1; i < 256; i++) {
        const h = data[i] * H;
        ctx2.fillRect(i * bw, H - h, bw + 0.5, h);
      }
    }

    // Shadow clip marker
    if (stats.length > 0) {
      const u0 = computeUniforms(stats, enabled, level)[0];
      if (u0 && u0.shadows > 0) {
        const sx = u0.shadows * W;
        ctx2.strokeStyle = "rgba(255,200,50,0.85)";
        ctx2.lineWidth = 1;
        ctx2.setLineDash([2, 2]);
        ctx2.beginPath(); ctx2.moveTo(sx, 0); ctx2.lineTo(sx, H); ctx2.stroke();
        ctx2.setLineDash([]);
      }
    }

    ctx2.strokeStyle = "rgba(255,255,255,0.12)";
    ctx2.lineWidth = 0.5;
    ctx2.strokeRect(0, 0, W, H);
  }

  // ── Annotations ───────────────────────────────────────────────────────────

  // True when we have enough WCS data to attempt annotation projection.
  let canAnnotate = $derived(
    !!(qualityFrame?.wcsSolved ||
      (fitsHeader?.ra && fitsHeader.pixelScale > 0))
  );

  $effect(() => {
    if (!showAnnotations || !canAnnotate || !rawInfo) return;
    if (annotations.length > 0) return;
    annotationsLoading = true;
    const ra    = qualityFrame?.wcsSolved ? qualityFrame.ra         : (fitsHeader?.ra  ?? 0);
    const dec   = qualityFrame?.wcsSolved ? qualityFrame.dec        : (fitsHeader?.dec ?? 0);
    const scale = qualityFrame?.wcsSolved ? qualityFrame.pixelScale : (fitsHeader?.pixelScale ?? 0);
    const rot   = qualityFrame?.wcsSolved ? qualityFrame.rotation   : (fitsHeader?.rotation ?? 0);
    GetAnnotations(ra, dec, scale, rot, rawInfo.width, rawInfo.height).then((res) => {
      annotations = res ?? [];
      annotationsLoading = false;
    }).catch(() => { annotationsLoading = false; });
  });

  // Convert image-space pixel coordinates to viewport-space coordinates.
  // The canvas uses max-width/max-height CSS (scales down to fit viewport, preserves AR),
  // followed by a translate+scale CSS transform for zoom/pan.
  function imgToViewport(imgX: number, imgY: number): { x: number; y: number } {
    if (!rawInfo || viewportW === 0 || viewportH === 0) return { x: -9999, y: -9999 };
    const cssScale = Math.min(viewportW / rawInfo.width, viewportH / rawInfo.height, 1);
    const displayW = rawInfo.width  * cssScale;
    const displayH = rawInfo.height * cssScale;
    // Canvas center in viewport (before zoom/pan)
    const cx = viewportW / 2;
    const cy = viewportH / 2;
    // Image pixel → offset from canvas center in CSS px
    const dx = (imgX - rawInfo.width  / 2) * cssScale;
    const dy = (imgY - rawInfo.height / 2) * cssScale;
    void displayW; void displayH; // used implicitly via cssScale
    return {
      x: cx + dx * zoom + panX,
      y: cy + dy * zoom + panY,
    };
  }

  // ── Wheel zoom ────────────────────────────────────────────────────────────

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.15 : 0.87;
    const newZoom = Math.max(0.1, Math.min(1.85, zoom * factor));
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const mx = e.clientX - rect.left - rect.width  / 2;
    const my = e.clientY - rect.top  - rect.height / 2;
    panX = mx - ((mx - panX) * newZoom) / zoom;
    panY = my - ((my - panY) * newZoom) / zoom;
    zoom = newZoom;

    if (upgradeTimer) clearTimeout(upgradeTimer);
    upgradeTimer = setTimeout(checkResolution, 450);
  }

  // Upgrades the WebGL texture to a higher resolution when the user has zoomed
  // in past the point where the current texture provides native pixel quality.
  async function checkResolution() {
    upgradeTimer = null;
    if (!rawInfo || !entry || !viewportW) return;
    const cssScale = Math.min(viewportW / rawInfo.width, viewportH / rawInfo.height, 1);
    const effective = zoom * cssScale;

    if (!(effective > 1.3 && loadedMaxSize <= 768)) return;
    const neededSize = 2048;

    const id = ++zoomReqId;
    let result;
    try {
      result = await GeneratePreviewRawSized(entry.path, neededSize);
    } catch {
      return;
    }
    if (id !== zoomReqId) return; // superseded

    const bin = atob(result.data);
    const u8 = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) u8[i] = bin.charCodeAt(i);
    const f32 = new Float32Array(u8.buffer);

    rawInfo = { width: result.width, height: result.height, channels: result.channels, stats: rawInfo.stats };
    canvas.width  = result.width;
    canvas.height = result.height;
    loadedMaxSize = neededSize;

    if (gl && glTex && !gl.isContextLost()) {
      gl.bindTexture(gl.TEXTURE_2D, glTex);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA32F, result.width, result.height, 0, gl.RGBA, gl.FLOAT, f32);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    }
    renderGL();
  }

  function onPanStart(e: MouseEvent) {
    if (e.button !== 0) return;
    isPanning = true;
    panStartX = e.clientX - panX;
    panStartY = e.clientY - panY;
  }
  function onPanMove(e: MouseEvent) { if (!isPanning) return; panX = e.clientX - panStartX; panY = e.clientY - panStartY; }
  function onPanEnd()               { isPanning = false; }
</script>

<div class="preview-pane">
  <div class="preview-titlebar">
    <span class="preview-filename" title={entry.path}>{entry.name}</span>
    <div class="preview-controls">
      <span class="zoom-label">{Math.round(zoom * 100)}%</span>
      <button class="tool-btn" onclick={resetView} title="Fit to window (double-click image)">Fit</button>

      {#if !isProcessed}
        <div class="stretch-group">
          <button
            class="tool-btn"
            class:active={stretchEnabled}
            onclick={() => { stretchEnabled = !stretchEnabled; applyStretch(); }}
            title="Toggle autostretch">Stretch</button>
          {#if stretchEnabled}
            <button class="tool-btn preset" class:active={stretchLevel === 1} onclick={() => setStretch(1)}>Gentle</button>
            <button class="tool-btn preset" class:active={stretchLevel === 2} onclick={() => setStretch(2)}>Normal</button>
            <button class="tool-btn preset" class:active={stretchLevel === 3} onclick={() => setStretch(3)}>Strong</button>
          {/if}
        </div>
      {/if}

      {#if rawInfo}
        {@const isColor = rawInfo.channels === 3}
        <div class="channel-group" class:ch-disabled={!isColor} title={isColor ? "" : "Channel split requires a color (OSC) image"}>
          <button class="tool-btn ch-btn"              class:active={channelMode === 0} disabled={!isColor} onclick={() => setChannelMode(0)}>RGB</button>
          <button class="tool-btn ch-btn ch-r" class:active={channelMode === 1} disabled={!isColor} onclick={() => setChannelMode(1)}>R</button>
          <button class="tool-btn ch-btn ch-g" class:active={channelMode === 2} disabled={!isColor} onclick={() => setChannelMode(2)}>G</button>
          <button class="tool-btn ch-btn ch-b" class:active={channelMode === 3} disabled={!isColor} onclick={() => setChannelMode(3)}>B</button>
        </div>
      {/if}

      {#if rawInfo}
        <button class="tool-btn" class:active={showHistogram} onclick={() => (showHistogram = !showHistogram)} title="Histogram overlay">Hist</button>
      {/if}

      {#if canAnnotate && rawInfo}
        <button class="tool-btn" class:active={showAnnotations} onclick={() => (showAnnotations = !showAnnotations)} title="Star / DSO annotations">✦ Labels</button>
      {/if}

      <button class="btn-icon small" onclick={onclose} title="Close preview">✕</button>
    </div>
  </div>

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="image-viewport"
    class:panning={isPanning}
    bind:this={viewportEl}
    onwheel={onWheel}
    onmousedown={onPanStart}
    onmousemove={onPanMove}
    onmouseup={onPanEnd}
    onmouseleave={onPanEnd}
    ondblclick={resetView}
  >
    <!-- Canvas kept in DOM so WebGL context survives re-renders -->
    <canvas
      bind:this={canvas}
      class="preview-canvas"
      class:visible={hasImage && !previewLoading}
      style="transform: translate({panX}px, {panY}px) scale({zoom});"
      draggable="false"
    ></canvas>

    <!-- Annotation overlay — absolute, viewport-space coordinates computed by imgToViewport() -->
    {#if showAnnotations && rawInfo && viewportW > 0}
      <svg class="annotation-svg" width={viewportW} height={viewportH}>
        {#if annotationsLoading}
          <text x={viewportW / 2} y={viewportH / 2} dominant-baseline="middle" text-anchor="middle" font-size="13" fill="rgba(255,200,50,0.7)">Loading annotations…</text>
        {:else if annotations.length === 0}
          <text x="8" y={viewportH - 8} font-size="10" fill="rgba(255,200,50,0.55)">No catalog objects in this field</text>
        {:else}
          {#each annotations as ann (`${ann.label}${ann.x}${ann.y}`)}
            {@const vp = imgToViewport(ann.x, ann.y)}
            {#if ann.type === "star"}
              <circle cx={vp.x} cy={vp.y} r="7" fill="none" stroke="rgba(136,196,255,0.75)" stroke-width="0.9"/>
              <text x={vp.x} y={vp.y + 16} font-size="10" fill="rgba(136,196,255,0.95)" text-anchor="middle" class="ann-lbl">{ann.label}</text>
            {:else}
              <line x1={vp.x - 9} y1={vp.y} x2={vp.x + 9} y2={vp.y} stroke="rgba(255,204,68,0.8)" stroke-width="0.9"/>
              <line x1={vp.x} y1={vp.y - 9} x2={vp.x} y2={vp.y + 9} stroke="rgba(255,204,68,0.8)" stroke-width="0.9"/>
              <text x={vp.x} y={vp.y + 17} font-size="10" fill="rgba(255,204,68,1)" text-anchor="middle" class="ann-lbl">{ann.label}</text>
            {/if}
          {/each}
        {/if}
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

  {#if fitsHeader}
    <div class="preview-meta">
      <div class="meta-section">
        <button class="meta-section-hdr" onclick={() => (basicCollapsed = !basicCollapsed)}>
          <span>Basic</span>
          <span class="meta-caret">{basicCollapsed ? "›" : "⌄"}</span>
        </button>
        {#if !basicCollapsed}
          {#each basicRows(fitsHeader) as row (row.key)}
            <div class="meta-row">
              <span class="meta-key">{row.key}</span>
              <span class="meta-val">{row.val}</span>
            </div>
          {/each}
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
              <div class="meta-row"><span class="meta-key">RA</span><span class="meta-val">{formatRA(qualityFrame.ra)}</span></div>
              <div class="meta-row"><span class="meta-key">Dec</span><span class="meta-val">{formatDec(qualityFrame.dec)}</span></div>
              {#if qualityFrame.pixelScale}
                <div class="meta-row"><span class="meta-key">Scale</span><span class="meta-val">{qualityFrame.pixelScale.toFixed(2)} "/px</span></div>
              {/if}
              {#if qualityFrame.rotation}
                <div class="meta-row"><span class="meta-key">Rotation</span><span class="meta-val">{qualityFrame.rotation.toFixed(1)}°</span></div>
              {/if}
            {:else if fitsHeader?.ra}
              <div class="meta-row"><span class="meta-key">RA</span><span class="meta-val">{formatRA(fitsHeader.ra)}</span></div>
              <div class="meta-row"><span class="meta-key">Dec</span><span class="meta-val">{formatDec(fitsHeader.dec)}</span></div>
            {:else if qualityFrame}
              <div class="meta-row stats-hint">
                <span class="meta-val">Not plate solved. Use <strong>✦ Analyze</strong> in the Library View.</span>
              </div>
            {/if}
          {/if}
        </div>
      {/if}
      <div class="meta-section">
        <button class="meta-section-hdr" onclick={() => (advancedCollapsed = !advancedCollapsed)}>
          <span>Advanced</span>
          <span class="meta-caret">{advancedCollapsed ? "›" : "⌄"}</span>
        </button>
        {#if !advancedCollapsed}
          {#each advancedRows(fitsHeader) as row (row.key)}
            <div class="meta-row">
              <span class="meta-key">{row.key}</span>
              <span class="meta-val">{row.val}</span>
            </div>
          {/each}
        {/if}
      </div>
      {#if qualityFrame}
        <div class="meta-section">
          <button class="meta-section-hdr" onclick={() => (statsCollapsed = !statsCollapsed)}>
            <span>Statistics</span>
            {#if qualityFrame.qualityAnalyzed}<span class="stats-badge">✦</span>{/if}
            <span class="meta-caret">{statsCollapsed ? "›" : "⌄"}</span>
          </button>
          {#if !statsCollapsed}
            {#if qualityFrame.qualityAnalyzed}
              <div class="meta-row"><span class="meta-key">Stars</span><span class="meta-val">{qualityFrame.starCount}</span></div>
              <div class="meta-row"><span class="meta-key">FWHM</span><span class="meta-val">{qualityFrame.fwhm.toFixed(2)} {qualityFrame.fwhmUnit || "px"}</span></div>
              {#if qualityFrame.background}
                <div class="meta-row"><span class="meta-key">Background</span><span class="meta-val">{qualityFrame.background.toFixed(1)} ADU</span></div>
              {/if}
              {#if qualityFrame.noise}
                <div class="meta-row"><span class="meta-key">Noise</span><span class="meta-val">{qualityFrame.noise.toFixed(2)} ADU</span></div>
              {/if}
              {#if qualityFrame.snr}
                <div class="meta-row"><span class="meta-key">SNR</span><span class="meta-val">{qualityFrame.snr.toFixed(1)}</span></div>
              {/if}
            {:else}
              <div class="meta-row stats-hint">
                <span class="meta-val">Not analyzed. Use <strong>✦ Analyze</strong> in the Library View.</span>
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
    justify-content: space-between;
    gap: 8px;
    padding: 6px 12px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .preview-filename {
    font-size: 0.8rem;
    font-family: "Consolas", "Fira Code", monospace;
    color: var(--text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }

  .preview-controls {
    display: flex;
    align-items: center;
    gap: 5px;
    flex-shrink: 0;
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .zoom-label {
    font-size: 0.75rem;
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
    min-width: 36px;
    text-align: right;
  }

  .stretch-group, .channel-group { display: flex; align-items: center; gap: 2px; }

  .ch-btn { min-width: 24px !important; padding: 2px 5px !important; font-weight: 700 !important; }
  .ch-r.active { color: #ff6060 !important; border-color: #ff6060 !important; }
  .ch-g.active { color: #50d050 !important; border-color: #50d050 !important; }
  .ch-b.active { color: #6098ff !important; border-color: #6098ff !important; }
  .ch-disabled { opacity: 0.38; cursor: not-allowed; }
  .ch-disabled .ch-btn { cursor: not-allowed; }

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
  .image-viewport.panning { cursor: grabbing; }

  .preview-canvas {
    max-width: 100%;
    max-height: 100%;
    display: block;
    user-select: none;
    pointer-events: none;
    transform-origin: center;
    will-change: transform;
    visibility: hidden;
  }
  .preview-canvas.visible { visibility: visible; }

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
    stroke: rgba(0,0,0,0.7);
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
    font-size: 0.875rem;
    gap: 8px;
    pointer-events: none;
  }
  .spinner {
    display: inline-block;
    animation: spin 1.2s linear infinite;
    color: var(--accent);
    font-size: 1.2rem;
  }
  @keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
  .preview-error {
    position: absolute;
    color: var(--danger);
    font-size: 0.82rem;
    background: #2a1020;
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
  .preview-meta::-webkit-scrollbar { width: 4px; }
  .preview-meta::-webkit-scrollbar-thumb { background: var(--border-accent); border-radius: 2px; }

  .meta-section { border-bottom: 1px solid var(--border); }
  .meta-section-hdr {
    display: flex; align-items: center; justify-content: space-between; width: 100%;
    background: transparent; border: none; cursor: pointer; padding: 5px 0 3px;
    color: var(--text-secondary); font-size: 0.68rem; font-weight: 700;
    text-transform: uppercase; letter-spacing: 0.08em;
  }
  .meta-section-hdr:hover { color: var(--text-primary); }
  .meta-caret { font-size: 0.8rem; opacity: 0.8; }

  .meta-row { display: flex; justify-content: space-between; padding: 2px 0; font-size: 0.79rem; border-bottom: 1px solid var(--border); }
  .meta-key { color: var(--text-secondary); width: 80px; flex-shrink: 0; }
  .meta-val { color: var(--text-primary); text-align: right; font-variant-numeric: tabular-nums; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

  .stats-badge { font-size: 0.6rem; color: var(--accent); margin-left: 4px; margin-right: auto; }
  .stats-hint { opacity: 0.7; font-style: italic; }
  .stats-hint .meta-val { text-align: left; white-space: normal; font-size: 0.73rem; }
</style>
