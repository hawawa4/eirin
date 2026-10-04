// @ts-check
import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";

// Deployed as a GitHub Pages project site. Override both for a custom domain
// (e.g. SITE_URL=https://eirin.example.org BASE_PATH=/).
const site = process.env.SITE_URL ?? "https://hawawa4.github.io";
const base = process.env.BASE_PATH ?? "/eirin";

export default defineConfig({
  site,
  base,
  trailingSlash: "ignore",
  // Compression drops the space when a line break precedes an inline tag
  // ("each frame.<em>Suggest"), which Prettier's wrapping produces constantly.
  compressHTML: false,
  integrations: [sitemap()],
  vite: {
    // Screenshots live in ../docs/img so the README and this site share them.
    server: { fs: { allow: [".."] } },
  },
});
