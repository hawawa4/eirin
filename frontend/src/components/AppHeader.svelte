<script lang="ts">
  import type { AppMode, Theme } from "../lib/types";
  import ModeSelector from "./ModeSelector.svelte";

  interface Props {
    rootFolder: string;
    appMode: AppMode;
    theme: Theme;
    onmodechange: (mode: AppMode) => void;
    onthemechange: (theme: Theme) => void;
  }

  let { rootFolder, appMode, theme, onmodechange, onthemechange }: Props = $props();

  const THEMES: { id: Theme; color: string; title: string }[] = [
    { id: "blue", color: "#7c9ef5", title: "Blue theme" },
    { id: "red", color: "#e07060", title: "Red (night) theme" },
    { id: "grey", color: "#9ba4b8", title: "Grey theme" },
  ];
</script>

<header>
  <span class="logo">✦ Eirin</span>

  <div class="mode-area">
    <ModeSelector mode={appMode} {onmodechange} {rootFolder} />
  </div>

  <div class="header-right">
    <div class="theme-toggle" role="group" aria-label="Theme">
      {#each THEMES as t (t.id)}
        <button
          class="theme-dot"
          class:active={theme === t.id}
          style="--dot-color: {t.color}"
          onclick={() => onthemechange(t.id)}
          title={t.title}
        ></button>
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
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--accent);
    letter-spacing: 0.05em;
    flex-shrink: 0;
  }

  .mode-area {
    flex: 1;
    display: flex;
    justify-content: center;
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
    gap: 7px;
    align-items: center;
  }

  .theme-dot {
    width: 13px;
    height: 13px;
    border-radius: 50%;
    background: var(--dot-color);
    border: 2px solid transparent;
    outline: none;
    cursor: pointer;
    padding: 0;
    opacity: 0.55;
    transition:
      opacity 0.15s,
      border-color 0.15s,
      transform 0.15s;
  }

  .theme-dot:hover {
    opacity: 0.9;
    transform: scale(1.2);
  }

  .theme-dot.active {
    border-color: var(--text-primary);
    opacity: 1;
  }
</style>
