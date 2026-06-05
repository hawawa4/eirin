import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { resolve } from "path";

const port = parseInt(process.env.WAILS_VITE_PORT || "9245");
const bindings = resolve(__dirname, "bindings/github.com/TaruDesigns/eirin/internal");

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      "$app": resolve(bindings, "app/app.ts"),
      "$models/app": resolve(bindings, "app/models.ts"),
      "$models/fits": resolve(bindings, "fits/models.ts"),
      "$models/catalog": resolve(bindings, "catalog/models.ts"),
      "$models/store": resolve(bindings, "store/models.ts"),
      "$models/importer": resolve(bindings, "importer/models.ts"),
      "$models/siril": resolve(bindings, "siril/models.ts"),
    },
  },
  server: {
    port,
    strictPort: true,
    host: "0.0.0.0",
  },
});
