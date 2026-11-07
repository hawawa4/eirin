<script lang="ts">
  import type { GalleryFormat, GalleryInfo, GalleryView } from "./types";

  interface Props {
    info: GalleryInfo;
    view: GalleryView;
    /** Some pictures have sky coordinates, so the atlas has something to show. */
    hasSky: boolean;
    onnavigate: (view: GalleryView) => void;
    /** The format being shown, how many pictures each has, and the switch. */
    format: GalleryFormat;
    counts: Record<GalleryFormat, number>;
    onformat: (format: GalleryFormat) => void;
  }

  let { info, view, hasSky, onnavigate, format, counts, onformat }: Props = $props();

  const FORMATS: { id: GalleryFormat; label: string; hint: string }[] = [
    { id: "fits", label: "FITS", hint: "The processed FITS files" },
    { id: "raster", label: "PNG / JPEG", hint: "The PNG / JPEG / TIFF exports" },
  ];

  let tabs = $derived<{ id: GalleryView; label: string; disabled?: boolean }[]>([
    { id: "grid", label: "Gallery" },
    { id: "atlas", label: "Sky Atlas", disabled: !hasSky },
    { id: "slideshow", label: "Slideshow", disabled: info.images.length === 0 },
  ]);
</script>

<header>
  <button class="title" onclick={() => onnavigate("grid")} title="Back to the gallery">
    {info.title}
  </button>

  <nav aria-label="Views">
    {#each tabs as t (t.id)}
      <button
        class="tab"
        class:active={view === t.id}
        aria-current={view === t.id ? "page" : undefined}
        disabled={t.disabled}
        title={t.disabled ? "Nothing to show here yet" : undefined}
        onclick={() => onnavigate(t.id)}>{t.label}</button
      >
    {/each}
  </nav>

  {#if counts.fits > 0 && counts.raster > 0}
    <div class="seg" role="group" aria-label="Show pictures as">
      {#each FORMATS as f (f.id)}
        <button
          class="seg-btn"
          class:active={format === f.id}
          aria-pressed={format === f.id}
          title="{f.hint} ({counts[f.id]})"
          onclick={() => onformat(f.id)}>{f.label}</button
        >
      {/each}
    </div>
  {/if}

  {#if info.homeUrl}
    <a class="home" href={info.homeUrl}>{info.homeLabel}</a>
  {/if}
</header>

<style>
  header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px 18px;
    padding: 8px 20px;
    min-height: 52px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }

  .title {
    font: inherit;
    font-size: var(--fs-lg);
    font-weight: 600;
    letter-spacing: 0.05em;
    color: var(--accent);
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    text-align: left;
    /* Takes the free space, so the tabs sit to the right of it. */
    margin-right: auto;
  }

  nav {
    display: flex;
    gap: 4px;
  }

  .tab {
    font: inherit;
    font-size: var(--fs-md);
    padding: 6px 14px;
    border-radius: 6px;
    border: 1px solid transparent;
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
    transition:
      color 0.15s,
      background 0.15s;
  }
  .tab:hover:not(:disabled) {
    color: var(--text-primary);
    background: var(--bg-row-hover);
  }
  .tab.active {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--accent-contrast);
  }
  .tab:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .seg {
    display: flex;
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }
  .seg-btn {
    font: inherit;
    font-size: var(--fs-sm);
    padding: 5px 10px;
    border: none;
    background: none;
    color: var(--text-secondary);
    cursor: pointer;
  }
  .seg-btn + .seg-btn {
    border-left: 1px solid var(--border);
  }
  .seg-btn:hover:not(.active) {
    color: var(--text-primary);
    background: var(--bg-row-hover);
  }
  .seg-btn.active {
    background: var(--accent);
    color: var(--accent-contrast);
  }
  .seg-btn:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }
  .tab:focus-visible,
  .title:focus-visible,
  .home:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }

  .home {
    font-size: var(--fs-sm);
    color: var(--text-secondary);
    text-decoration: none;
    border-bottom: 1px solid var(--border-accent);
  }
  .home:hover {
    color: var(--text-primary);
  }

  @media (max-width: 560px) {
    header {
      padding: 8px 14px;
    }
    nav {
      order: 3;
      width: 100%;
    }
    .tab {
      flex: 1;
    }
  }
</style>
