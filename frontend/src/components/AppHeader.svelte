<script lang="ts">
  import type { AppMode } from "../lib/types";
  import ModeSelector from "./ModeSelector.svelte";

  interface Props {
    rootFolder: string;
    appMode: AppMode;
    onmodechange: (mode: AppMode) => void;
  }

  let { rootFolder, appMode, onmodechange }: Props = $props();
</script>

<header>
  <span class="logo">✦ Eirin</span>

  <div class="mode-area">
    <ModeSelector mode={appMode} {onmodechange} {rootFolder} />
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
</style>
