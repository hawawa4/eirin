// ── UI scale ────────────────────────────────────────────────────────────────
// Root font-size follows ui.uiScale; Ctrl/Cmd + "=" / "+" / "-" / "0" step or reset it.

import { ui, UI_SCALES, DEFAULT_UI_SCALE } from "./uiState.svelte";

/** Moves ui.uiScale one step up (+1) or down (-1) within UI_SCALES. */
export function stepUiScale(dir: 1 | -1): void {
  const i = UI_SCALES.indexOf(ui.uiScale);
  const next = UI_SCALES[Math.max(0, Math.min(UI_SCALES.length - 1, i + dir))];
  if (next !== undefined) ui.uiScale = next;
}

export function resetUiScale(): void {
  ui.uiScale = DEFAULT_UI_SCALE;
}

function onKeydown(e: KeyboardEvent) {
  if (!(e.ctrlKey || e.metaKey) || e.altKey) return;
  switch (e.key) {
    case "=":
    case "+":
      stepUiScale(1);
      break;
    case "-":
    case "_":
      stepUiScale(-1);
      break;
    case "0":
      resetUiScale();
      break;
    default:
      return;
  }
  e.preventDefault();
}

/**
 * Applies ui.uiScale to <html> font-size (reactively) and installs the zoom shortcuts.
 * Returns a cleanup function — call from onMount.
 */
export function installUiScaleShortcuts(): () => void {
  const stopEffect = $effect.root(() => {
    $effect(() => {
      document.documentElement.style.fontSize = `${ui.uiScale}px`;
    });
  });
  window.addEventListener("keydown", onKeydown);
  return () => {
    window.removeEventListener("keydown", onKeydown);
    stopEffect();
  };
}
