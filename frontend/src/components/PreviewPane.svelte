<script lang="ts">
  import { onMount } from "svelte";
  import { untrack } from "svelte";
  import type { app } from "../../wailsjs/go/models";
  import { GeneratePreviewRaw, ReadFITSHeader } from "../../wailsjs/go/app/App.js";
  import { basicRows, advancedRows } from "../lib/utils";

  interface Props {
    entry: app.EnrichedFileEntry;
    stretchEnabled: boolean;
    stretchLevel: number;
    basicCollapsed: boolean;
    advancedCollapsed: boolean;
    onclose: () => void;
  }

  let {
    entry,
    stretchEnabled = $bindable(),
    stretchLevel = $bindable(),
    basicCollapsed = $bindable(),
    advancedCollapsed = $bindable(),
    onclose,
  }: Props = $props();

  // ── Preview load state ───────────────────────────────────────────────────
  let previewLoading = $state(false);
  let previewError = $state("");
  let fitsHeader = $state<app.FITSHeader | null>(null);
  let previewReqId = 0;
  let hasImage = $state(false);

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
  }

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
  } | null = null;
  let rawInfo: {
    width: number;
    height: number;
    channels: number;
    stats: app.ChannelStats[];
  } | null = null;

  // ── GLSL shaders ──────────────────────────────────────────────────────────
  //
  // Vertex: maps the [-1,1] clip-space quad to UV [0,1].
  // The Y-flip (1.0 - y) compensates for OpenGL's bottom-left texture origin
  // vs. the backend's top-left row order.
  const VS = `#version 300 es
in vec2 aPos;
out vec2 vUv;
void main() {
  vUv = vec2(aPos.x * 0.5 + 0.5, 1.0 - (aPos.y * 0.5 + 0.5));
  gl_Position = vec4(aPos, 0.0, 1.0);
}`;

  // Fragment: applies shadow-clip + optional MTF curve per channel.
  // The MTF formula is identical to Siril's: MTF(m,x) = (m-1)x / ((2m-1)x - m)
  const FS = `#version 300 es
precision highp float;
uniform sampler2D uTex;
uniform vec3  uShadows;
uniform vec3  uMidtones;
uniform bool  uLinear;
uniform int   uChannels;
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
  } else {
    float r = applyStretch(uShadows.x, uMidtones.x, raw.r);
    float g = applyStretch(uShadows.y, uMidtones.y, raw.g);
    float b = applyStretch(uShadows.z, uMidtones.z, raw.b);
    fragColor = vec4(r, g, b, 1.0);
  }
}`;

  // ── Stretch math (mirrors the Go backend) ─────────────────────────────────

  function mtfMidtone(target: number, x: number): number {
    if (x === 0) return 0;
    const d = x * (1 - 2 * target) + target;
    if (d === 0) return 0;
    return (x * (1 - target)) / d;
  }

  interface StretchUniforms {
    shadows: number;
    midtone: number;
    linear: boolean;
  }

  function computeUniforms(
    stats: app.ChannelStats[],
    enabled: boolean,
    level: number,
  ): StretchUniforms[] {
    const presets = [
      { shadowsFactor: -1.25, targetBG: 0.1 }, // level 1 – gentle
      { shadowsFactor: -2.8, targetBG: 0.25 }, // level 2 – normal (Siril default)
      { shadowsFactor: -4.0, targetBG: 0.4 }, // level 3 – strong
    ];

    return stats.map((s) => {
      const { median: med, sigma: sig } = s;

      if (!enabled) {
        // Linear auto-stretch: clip shadows at −2.8σ, scale linearly (no MTF).
        return { shadows: Math.max(0, med - 2.8 * sig), midtone: 0.5, linear: true };
      }

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
    if (!gl || !program || !glTex || !glU || !rawInfo) return;

    const uniforms = computeUniforms(rawInfo.stats, stretchEnabled, stretchLevel);
    const u0 = uniforms[0] ?? { shadows: 0, midtone: 0.5, linear: true };
    const u1 = uniforms[1] ?? u0;
    const u2 = uniforms[2] ?? u0;

    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.useProgram(program);

    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, glTex);
    gl.uniform1i(glU.uTex, 0);

    gl.uniform3fv(glU.uShadows, [u0.shadows, u1.shadows, u2.shadows]);
    gl.uniform3fv(glU.uMidtones, [u0.midtone, u1.midtone, u2.midtone]);
    gl.uniform1i(glU.uLinear, u0.linear ? 1 : 0);
    gl.uniform1i(glU.uChannels, rawInfo.channels);

    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
  }

  // ── WebGL init (runs after DOM mount) ─────────────────────────────────────

  onMount(() => {
    const ctx = canvas.getContext("webgl2");
    if (!ctx) {
      previewError = "WebGL2 not supported";
      return;
    }
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
    if (!vs || !fs) {
      previewError = "Shader compile failed";
      return;
    }

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

    // Fullscreen triangle-strip quad
    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, 1, 1]), gl.STATIC_DRAW);
    const aPos = gl.getAttribLocation(program, "aPos");
    gl.enableVertexAttribArray(aPos);
    gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

    glTex = gl.createTexture();

    glU = {
      uTex: gl.getUniformLocation(program, "uTex")!,
      uShadows: gl.getUniformLocation(program, "uShadows")!,
      uMidtones: gl.getUniformLocation(program, "uMidtones")!,
      uLinear: gl.getUniformLocation(program, "uLinear")!,
      uChannels: gl.getUniformLocation(program, "uChannels")!,
    };
  });

  // ── Load when entry changes ───────────────────────────────────────────────

  $effect(() => {
    const e = entry;
    previewLoading = true;
    previewError = "";
    fitsHeader = null;
    rawInfo = null;
    hasImage = false;
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

          // Decode base64 float32 RGBA data
          const bin = atob(result.data);
          const u8 = new Uint8Array(bin.length);
          for (let i = 0; i < bin.length; i++) u8[i] = bin.charCodeAt(i);
          const f32 = new Float32Array(u8.buffer);

          rawInfo = {
            width: result.width,
            height: result.height,
            channels: result.channels,
            stats: result.stats,
          };

          canvas.width = result.width;
          canvas.height = result.height;

          if (gl && glTex) {
            gl.bindTexture(gl.TEXTURE_2D, glTex);
            gl.texImage2D(
              gl.TEXTURE_2D,
              0,
              gl.RGBA32F,
              result.width,
              result.height,
              0,
              gl.RGBA,
              gl.FLOAT,
              f32,
            );
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
          }

          // Render with whatever stretch is currently active
          const snapSe = se;
          const snapSl = sl;
          // Temporarily point renderGL at the just-received stats
          renderGLWith(rawInfo.stats, snapSe, snapSl);
          hasImage = true;
        } else {
          previewError =
            (rawResult as PromiseRejectedResult).reason?.toString() ?? "Preview failed";
        }
      },
    );
  });

  // renderGLWith is a version of renderGL that takes explicit params so we can
  // call it safely before the reactive `stretchEnabled/stretchLevel` settle.
  function renderGLWith(stats: app.ChannelStats[], enabled: boolean, level: number) {
    if (!gl || !program || !glTex || !glU || !rawInfo) return;
    const uniforms = computeUniforms(stats, enabled, level);
    const u0 = uniforms[0] ?? { shadows: 0, midtone: 0.5, linear: true };
    const u1 = uniforms[1] ?? u0;
    const u2 = uniforms[2] ?? u0;
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.useProgram(program);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, glTex);
    gl.uniform1i(glU.uTex, 0);
    gl.uniform3fv(glU.uShadows, [u0.shadows, u1.shadows, u2.shadows]);
    gl.uniform3fv(glU.uMidtones, [u0.midtone, u1.midtone, u2.midtone]);
    gl.uniform1i(glU.uLinear, u0.linear ? 1 : 0);
    gl.uniform1i(glU.uChannels, rawInfo.channels);
    gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
  }

  // ── Stretch controls (instant — no backend round-trip) ────────────────────

  function applyStretch() {
    renderGL();
  }

  function setStretch(level: number) {
    stretchLevel = level;
    renderGL();
  }

  // ── Wheel zoom ────────────────────────────────────────────────────────────
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
  }
  function onPanEnd() {
    isPanning = false;
  }
</script>

<div class="preview-pane">
  <div class="preview-titlebar">
    <span class="preview-filename" title={entry.path}>{entry.name}</span>
    <div class="preview-controls">
      <span class="zoom-label">{Math.round(zoom * 100)}%</span>
      <button class="tool-btn" onclick={resetView} title="Fit to window (or double-click image)"
        >Fit</button
      >
      <div class="stretch-group">
        <button
          class="tool-btn"
          class:active={stretchEnabled}
          onclick={() => {
            stretchEnabled = !stretchEnabled;
            applyStretch();
          }}
          title="Toggle autostretch">Stretch</button
        >
        {#if stretchEnabled}
          <button
            class="tool-btn preset"
            class:active={stretchLevel === 1}
            onclick={() => setStretch(1)}>Gentle</button
          >
          <button
            class="tool-btn preset"
            class:active={stretchLevel === 2}
            onclick={() => setStretch(2)}>Normal</button
          >
          <button
            class="tool-btn preset"
            class:active={stretchLevel === 3}
            onclick={() => setStretch(3)}>Strong</button
          >
        {/if}
      </div>
      <button class="btn-icon small" onclick={onclose} title="Close preview">✕</button>
    </div>
  </div>

  <div
    class="image-viewport"
    class:panning={isPanning}
    onwheel={onWheel}
    onmousedown={onPanStart}
    onmousemove={onPanMove}
    onmouseup={onPanEnd}
    onmouseleave={onPanEnd}
    ondblclick={resetView}
  >
    <!-- Canvas is always in the DOM so WebGL context survives re-renders -->
    <canvas
      bind:this={canvas}
      class="preview-canvas"
      class:visible={hasImage && !previewLoading}
      style="transform: translate({panX}px, {panY}px) scale({zoom});"
      draggable="false"
    ></canvas>

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
    gap: 6px;
    flex-shrink: 0;
  }

  .zoom-label {
    font-size: 0.75rem;
    color: var(--text-secondary);
    font-variant-numeric: tabular-nums;
    min-width: 36px;
    text-align: right;
  }

  .stretch-group {
    display: flex;
    align-items: center;
    gap: 3px;
  }

  /* ── Image viewport ───────────────────────────────────────────────────── */

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

  .preview-canvas {
    max-width: 100%;
    max-height: 100%;
    display: block;
    user-select: none;
    pointer-events: none;
    transform-origin: center;
    will-change: transform;
    /* Hidden until image data is ready — keeps the canvas in DOM for WebGL */
    visibility: hidden;
  }
  .preview-canvas.visible {
    visibility: visible;
  }

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
    font-size: 0.68rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
  .meta-section-hdr:hover {
    color: var(--text-primary);
  }
  .meta-caret {
    font-size: 0.8rem;
    opacity: 0.8;
  }

  .meta-row {
    display: flex;
    justify-content: space-between;
    padding: 2px 0;
    font-size: 0.79rem;
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
</style>
