<script lang="ts">
  // Full-canvas status overlays: loading, load error, nothing indexed, everything filtered out.
  interface Props {
    rootPath: string;
    loading: boolean;
    error: string;
    indexed: number;
    visible: number;
    onretry: () => void;
    onshowall: () => void;
    onscan?: () => void;
  }

  let { rootPath, loading, error, indexed, visible, onretry, onshowall, onscan }: Props = $props();
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
{:else if indexed === 0}
  <div class="atlas-status">
    <div class="status-card">
      <div class="icon" aria-hidden="true">◎</div>
      <p class="title">No frames with sky coordinates yet</p>
      <p class="detail">
        The atlas shows stacked and processed frames whose FITS headers contain WCS (plate-solve)
        coordinates. Scan your library to pick them up — frames without coordinates can be
        plate-solved with <strong>Analyze</strong> in the Library.
      </p>
      {#if onscan}
        <div class="actions">
          <button class="btn primary" onclick={onscan}>Scan library</button>
        </div>
      {/if}
    </div>
  </div>
{:else if visible === 0}
  <div class="atlas-status">
    <div class="status-card">
      <p class="title">
        {indexed} frame{indexed === 1 ? "" : "s"} hidden by the “Final images” filter
      </p>
      <p class="detail">None of your located frames is a processed or exported image yet.</p>
      <div class="actions">
        <button class="btn primary" onclick={onshowall}>Show all frames</button>
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
