<script lang="ts">
  import { levelFor, previewUrl } from "../lib/images";
  import { summaryLine } from "./describe";
  import type { GalleryImage, GalleryInfo } from "./types";

  interface Props {
    info: GalleryInfo;
    /** The pictures to show: the format the visitor picked. */
    images: GalleryImage[];
    onopen: (path: string) => void;
  }

  let { info, images, onopen }: Props = $props();

  /** Cards are about 300 px wide; the preview level that is sharp at that size. */
  const THUMB_LEVEL = levelFor(320);

  let paragraphs = $derived(info.about.split(/\n\s*\n/).filter((p) => p.trim()));
  let objects = $derived(new Set(images.map((i) => i.title)).size);
</script>

<div class="page">
  <section class="hero">
    <h1>{info.title}</h1>
    {#if info.subtitle}<p class="subtitle">{info.subtitle}</p>{/if}
    {#if paragraphs.length > 0}
      <div class="about">
        {#each paragraphs as p, i (i)}<p>{p}</p>{/each}
      </div>
    {/if}
    {#if images.length > 0}
      <p class="count">
        {images.length} picture{images.length === 1 ? "" : "s"}
        {#if objects > 1}· {objects} objects{/if}
      </p>
    {/if}
  </section>

  {#if images.length === 0}
    <p class="empty">No pictures yet.</p>
  {:else}
    <ul class="grid">
      {#each images as img (img.path)}
        {@const line = summaryLine(img)}
        <li>
          <button class="card" onclick={() => onopen(img.path)} aria-label="Open {img.title}">
            <img
              src={previewUrl(img.path, THUMB_LEVEL, 0)}
              alt={img.title}
              loading="lazy"
              decoding="async"
              draggable="false"
            />
            <span class="caption">
              <span class="name">{img.title}</span>
              {#if line}<span class="meta">{line}</span>{/if}
            </span>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .page {
    flex: 1;
    overflow-y: auto;
    padding: 0 clamp(14px, 4vw, 48px) 48px;
    background: radial-gradient(
      900px 340px at 50% -80px,
      color-mix(in srgb, var(--accent) 16%, transparent),
      transparent
    );
  }

  .hero {
    max-width: 760px;
    margin: 0 auto;
    padding: clamp(28px, 7vw, 72px) 0 clamp(22px, 4vw, 40px);
    text-align: center;
    user-select: text;
  }
  h1 {
    font-size: clamp(1.7rem, 5vw, 2.75rem);
    font-weight: 300;
    letter-spacing: 0.06em;
    line-height: 1.15;
  }
  .subtitle {
    margin-top: 10px;
    font-size: var(--fs-xl);
    color: var(--accent);
  }
  .about {
    margin-top: 18px;
    color: var(--text-secondary);
    line-height: 1.6;
    font-size: var(--fs-lg);
  }
  .about p + p {
    margin-top: 0.8em;
  }
  .count {
    margin-top: 20px;
    font-size: var(--fs-sm);
    color: var(--text-dim);
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }

  .empty {
    text-align: center;
    color: var(--text-secondary);
    padding: 40px 0;
  }

  .grid {
    list-style: none;
    max-width: 1500px;
    margin: 0 auto;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(min(100%, 260px), 1fr));
    gap: clamp(10px, 1.6vw, 18px);
  }

  .card {
    position: relative;
    display: block;
    width: 100%;
    aspect-ratio: 4 / 5;
    padding: 0;
    overflow: hidden;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--bg-panel);
    color: #fff;
    font: inherit;
    cursor: pointer;
    text-align: left;
    transition:
      border-color 0.2s,
      transform 0.2s;
  }
  .card:hover,
  .card:focus-visible {
    border-color: var(--accent);
    transform: translateY(-2px);
  }
  .card:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    display: block;
    transition: transform 0.5s;
  }
  .card:hover img {
    transform: scale(1.04);
  }

  .caption {
    position: absolute;
    inset: auto 0 0 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 36px 14px 12px;
    background: linear-gradient(to top, rgb(0 0 0 / 0.78), transparent);
    pointer-events: none;
  }
  .name {
    font-weight: 600;
    font-size: var(--fs-lg);
    overflow-wrap: anywhere;
  }
  .meta {
    font-size: var(--fs-sm);
    color: rgb(255 255 255 / 0.72);
  }

  @media (prefers-reduced-motion: reduce) {
    .card,
    img {
      transition: none;
    }
    .card:hover,
    .card:hover img {
      transform: none;
    }
  }
</style>
