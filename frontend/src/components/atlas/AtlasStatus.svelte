<script lang="ts">
  // Full-canvas status overlays: loading, load error, nothing indexed, everything filtered out.
  import { GetLibraryFrames } from "$app";
  import { isProjectFrameType } from "../../lib/projects/frames";

  interface Props {
    rootPath: string;
    loading: boolean;
    error: string;
    indexed: number;
    visible: number;
    /** The format being shown, and how many final images the other one has. */
    format: "fits" | "png";
    otherCount: number;
    onretry: () => void;
    onswitchformat: () => void;
    onscan?: () => void;
    /** Read-only server viewer: nothing to scan, the desktop app publishes the library. */
    readOnly?: boolean;
    scanning?: boolean;
    scanProgress?: { done: number; total: number } | null;
    onnavigate?: (mode: "import" | "library" | "projects" | "coverage") => void;
  }

  let {
    rootPath,
    loading,
    error,
    indexed,
    visible,
    format,
    otherCount,
    onretry,
    onswitchformat,
    onscan,
    readOnly = false,
    scanning = false,
    scanProgress = null,
    onnavigate,
  }: Props = $props();

  // With nothing to show, look at the library to say why (desktop only).
  let lib = $state<{ total: number; lights: number; finals: number } | null>(null);
  $effect(() => {
    lib = null;
    if (readOnly || loading || error || !rootPath || indexed > 0 || scanning) return;
    const root = rootPath;
    let stale = false;
    GetLibraryFrames(root)
      .then((frames) => {
        if (stale) return;
        let lights = 0;
        let finals = 0;
        for (const f of frames ?? []) {
          if (f.isRejected) continue;
          if (f.frameType === "light") lights++;
          else if (!isProjectFrameType(f.frameType)) finals++;
        }
        lib = { total: frames?.length ?? 0, lights, finals };
      })
      .catch(() => {
        /* fall back to the generic card */
      });
    return () => {
      stale = true;
    };
  });
</script>

{#if loading}
  <div class="atlas-status">
    <div class="status-card">
      <div class="spinner" aria-hidden="true"></div>
      <p>Loading sky atlas…</p>
    </div>
  </div>
{:else if error}
  <div class="atlas-status">
    <div class="status-card" role="alert">
      <p class="title error">Couldn't load the sky atlas</p>
      <p class="detail">{error}</p>
      <div class="actions">
        <button class="btn primary" onclick={onretry}>Retry</button>
      </div>
    </div>
  </div>
{:else if !rootPath}
  <div class="atlas-status">
    <div class="status-card">
      <div class="icon" aria-hidden="true">◎</div>
      <p class="title">No library folder selected</p>
      <p class="detail">
        Choose your library root folder in Settings to see your frames on the sky.
      </p>
    </div>
  </div>
{:else if indexed === 0 && scanning}
  <div class="atlas-status">
    <div class="status-card" role="status">
      <div class="spinner" aria-hidden="true"></div>
      <p class="title">Scanning your library…</p>
      <p class="detail">
        {#if scanProgress && scanProgress.total > 0}
          {scanProgress.done.toLocaleString()} of {scanProgress.total.toLocaleString()} files read.
        {/if}
        Plate-solved images appear here when the scan finishes.
      </p>
    </div>
  </div>
{:else if indexed === 0}
  <div class="atlas-status">
    <div class="status-card">
      <div class="icon" aria-hidden="true">◎</div>
      {#if readOnly}
        <p class="title">No final images with sky coordinates yet</p>
        <p class="detail">
          The atlas shows the plate-solved final images from the library the desktop app publishes.
          Once it has published a snapshot with some, they'll appear here.
        </p>
      {:else if lib && lib.total === 0}
        <p class="title">Welcome to Eirin</p>
        <p class="detail">
          This is the Sky Atlas: your stacked and processed images, placed where they are on the
          sky. Your library is empty so far — import from your Seestar, or scan the library folder
          if it already holds data.
        </p>
        <div class="actions">
          {#if onnavigate}<button class="btn primary" onclick={() => onnavigate("import")}
              >Import from Seestar</button
            >{/if}
          {#if onscan}<button class="btn" onclick={onscan}>Scan library folder</button>{/if}
        </div>
      {:else if lib && lib.finals === 0}
        <p class="title">No stacked or processed images yet</p>
        <p class="detail">
          You have {lib.lights.toLocaleString()} light frame{lib.lights === 1 ? "" : "s"}. The atlas
          shows final images, so stack some in a project first. Coverage already shows where your
          lights point.
        </p>
        <div class="actions">
          {#if onnavigate}
            <button class="btn primary" onclick={() => onnavigate("library")}>Open Library</button>
            <button class="btn" onclick={() => onnavigate("coverage")}>See Coverage</button>
          {/if}
        </div>
      {:else if lib}
        <p class="title">Your final images aren't plate-solved</p>
        <p class="detail">
          {lib.finals.toLocaleString()} stacked or processed image{lib.finals === 1
            ? " has"
            : "s have"}
          no sky coordinates. In the Library, pick the <strong>Stacked</strong> type, then
          right-click a group and choose <strong>Analyze</strong> to plate-solve with Siril.
        </p>
        <div class="actions">
          {#if onnavigate}<button class="btn primary" onclick={() => onnavigate("library")}
              >Open Library</button
            >{/if}
          {#if onscan}<button class="btn" onclick={onscan}>Rescan</button>{/if}
        </div>
      {:else}
        <p class="title">No final images with sky coordinates yet</p>
        <p class="detail">
          The atlas shows your processed images that have WCS (plate-solve) coordinates. Scan your
          library to pick them up — images without coordinates can be plate-solved with
          <strong>Analyze</strong> in the Library.
        </p>
        {#if onscan}
          <div class="actions">
            <button class="btn primary" onclick={onscan}>Scan library</button>
          </div>
        {/if}
      {/if}
    </div>
  </div>
{:else if visible === 0}
  {@const shownName = format === "fits" ? "FITS" : "PNG"}
  {@const otherName = format === "fits" ? "PNG" : "FITS"}
  <div class="atlas-status">
    <div class="status-card">
      <p class="title">No final {shownName} images</p>
      <p class="detail">
        {otherCount} final {otherName} image{otherCount === 1 ? "" : "s"} can be shown instead.
      </p>
      <div class="actions">
        <button class="btn primary" onclick={onswitchformat}>Show {otherName}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .atlas-status {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 15;
    pointer-events: none;
    background: var(--atlas-backdrop);
  }
  .status-card {
    pointer-events: auto;
    max-width: 420px;
    padding: 20px 24px;
    background: var(--bg-panel);
    border: 1px solid var(--border-accent);
    border-radius: 8px;
    box-shadow: 0 8px 28px rgba(0, 0, 0, 0.45);
    text-align: center;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }
  p {
    margin: 0;
  }
  .icon {
    font-size: 2.25rem;
    color: var(--accent);
    line-height: 1;
  }
  .title {
    font-size: var(--fs-lg);
    font-weight: 600;
    color: var(--text-primary);
  }
  .title.error {
    color: var(--danger);
  }
  .detail {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    line-height: 1.5;
    word-break: break-word;
  }
  .detail strong {
    color: var(--text-primary);
  }
  .actions {
    margin-top: 6px;
    display: flex;
    gap: 8px;
  }
  .btn {
    padding: 6px 16px;
    border-radius: 5px;
    font-size: var(--fs-sm);
    font-weight: 600;
    cursor: pointer;
    border: 1px solid var(--border-accent);
    background: var(--bg-row);
    color: var(--text-primary);
  }
  .btn.primary {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }
  .btn:hover {
    filter: brightness(1.1);
  }
  .spinner {
    width: 32px;
    height: 32px;
    border: 3px solid color-mix(in srgb, var(--accent) 25%, transparent);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
  }
  .status-card p:not(.title):not(.detail) {
    font-size: var(--fs-md);
    color: var(--text-secondary);
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
