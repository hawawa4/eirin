// ── Immersive (fullscreen) mode ─────────────────────────────────────────────
// Used by the slideshow and the Sky Atlas tour: the window goes fullscreen and
// App hides its header, leaving only the view. The desktop app drives the
// native window (Wails); the browser-based server viewer uses the DOM
// Fullscreen API, which the user can also leave with Esc.

import { Window } from "@wailsio/runtime";

export const immersive = $state({ on: false });

/** Whether the native window (not the DOM) is the one that was made fullscreen. */
let nativeWindow = false;

if (typeof document !== "undefined") {
  // Esc (or the browser UI) can leave DOM fullscreen without going through us.
  document.addEventListener("fullscreenchange", () => {
    if (!nativeWindow && immersive.on && !document.fullscreenElement) immersive.on = false;
  });
}

export function enterImmersive(desktop: boolean): void {
  immersive.on = true;
  nativeWindow = desktop;
  if (desktop) {
    void Window.Fullscreen().catch(() => {
      /* the header stays hidden; leaving still works */
    });
  } else {
    void document.documentElement.requestFullscreen?.().catch(() => {
      /* denied by the browser; the header stays hidden */
    });
  }
}

export function exitImmersive(): void {
  if (!immersive.on) return;
  immersive.on = false;
  if (nativeWindow) {
    void Window.UnFullscreen().catch(() => {});
  } else if (document.fullscreenElement) {
    void document.exitFullscreen().catch(() => {});
  }
}

export function toggleImmersive(desktop: boolean): void {
  if (immersive.on) exitImmersive();
  else enterImmersive(desktop);
}
