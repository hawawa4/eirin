# Eirin landing page

Static site for Eirin, built with [Astro](https://astro.build) and deployed to GitHub Pages by `.github/workflows/landingpage.yml`.

```
just install-landing
just landing-dev      # http://localhost:4321/eirin/
just landing-build    # static output in landingpage/dist/
just landing-check    # astro check + prettier
```

- The page is `src/pages/index.astro`; `src/components/` has the screenshot and feature-row components.
- Screenshots are imported from `../docs/img/` (shared with the main README). Astro generates resized AVIF/WebP versions at build time, so commit full-resolution PNGs there.
- Clips in `../docs/vid/` (WebM) are imported with `?url` and rendered by `src/components/VideoShot.astro`, which shows a screenshot as the poster and only downloads and plays the clip once it scrolls into view.
- `site` and `base` default to the GitHub Pages project URL. For a custom domain, build with `SITE_URL=https://example.org BASE_PATH=/`.
