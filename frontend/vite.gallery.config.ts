// The public gallery's bundle (the server build): its own entry under
// gallery/, built to dist-gallery/, which only the server binary embeds. It
// reuses the app's Sky Atlas and Slideshow components, but `$app` — which in
// the desktop app is the generated bindings of the whole App — is replaced by
// a small module that only knows the gallery's service (src/gallery/appShim.ts).

import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve } from "path";

export default defineConfig({
  root: resolve(__dirname, "gallery"),
  publicDir: resolve(__dirname, "public"),
  plugins: [svelte({ configFile: resolve(__dirname, "svelte.config.js") })],
  resolve: {
    alias: {
      $app: resolve(__dirname, "src/gallery/appShim.ts"),
    },
  },
  build: {
    outDir: resolve(__dirname, "dist-gallery"),
    emptyOutDir: true,
  },
});
