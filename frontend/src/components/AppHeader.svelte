<script lang="ts">
  import type { AppMode, Theme } from "../lib/types";
  import ModeSelector from "./ModeSelector.svelte";

  interface Props {
    rootFolder: string;
    appMode: AppMode;
    theme: Theme;
    readOnly: boolean;
    onmodechange: (mode: AppMode) => void;
    onthemechange: (theme: Theme) => void;
  }

  let { rootFolder, appMode, theme, readOnly, onmodechange, onthemechange }: Props = $props();

  // Swatch colours are each theme's own accent, so they can't come from the
  // (current-theme) CSS tokens.
  const THEMES: { id: Theme; color: string; title: string }[] = [
    { id: "blue", color: "#7c9ef5", title: "Blue theme" },
    { id: "red", color: "#e07060", title: "Red (night) theme" },
    { id: "grey", color: "#9ba4b8", title: "Grey theme" },
  ];
</script>

<header>
  <span class="logo">✦ Eirin</span>

  <div class="mode-area">
    <ModeSelector mode={appMode} {onmodechange} {rootFolder} {readOnly} />
  </div>

  <div class="header-right">
    <div class="theme-toggle" role="group" aria-label="Theme">
      {#each THEMES as t (t.id)}
        <button
          class="theme-btn"
          class:active={theme === t.id}
          style="--dot-color: {t.color}"
          onclick={() => onthemechange(t.id)}
          title={t.title}
          aria-label={t.title}
          aria-pressed={theme === t.id}
        >
          <span class="theme-dot" aria-hidden="true"></span>
        </button>
      {/each}
    </div>
  </div>
</header>

<style>
  header {
    display: flex;
    align-items: center;
    padding: 0 20px;
    height: 52px;
    background: var(--bg-panel);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    -webkit-app-region: drag;
    gap: 16px;
  }

  .logo {
    font-size: var(--fs-lg);
    font-weight: 600;
    color: var(--accent);
    letter-spacing: 0.05em;
    flex-shrink: 0;
  }

  .mode-area {
    flex: 1;
    display: flex;
    justify-content: center;
    min-width: 0;
    -webkit-app-region: no-drag;
  }

  .header-right {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    -webkit-app-region: no-drag;
  }

  .theme-toggle {
    display: flex;
    gap: 2px;
    align-items: center;
  }

  /* 22px hit target around a 13px swatch. */
  .theme-btn {
    width: 22px;
    height: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 50%;
    padding: 0;
    cursor: pointer;
  }

  .theme-dot {
    width: 13px;
    height: 13px;
    border-radius: 50%;
    background: var(--dot-color);
    border: 2px solid transparent;
    opacity: 0.55;
    transition:
      opacity 0.15s,
      border-color 0.15s,
      transform 0.15s;
  }

  .theme-btn:hover .theme-dot {
    opacity: 0.9;
    transform: scale(1.2);
  }

  .theme-btn.active .theme-dot {
    border-color: var(--text-primary);
    opacity: 1;
  }
</style>
