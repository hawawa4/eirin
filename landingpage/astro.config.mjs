// @ts-check
import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";

// Public URL, used for canonical links, the og:image and the sitemap. Served from
// the domain root by the Docker image (landingpage/Dockerfile); override with
// SITE_URL / BASE_PATH to host it elsewhere or under a sub-path.
const site = process.env.SITE_URL || "https://eirin.hawawa.org";
const base = process.env.BASE_PATH || "/";

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
