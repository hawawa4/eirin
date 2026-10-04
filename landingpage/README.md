# Eirin landing page

Static site for Eirin, built with [Astro](https://astro.build) and served at https://eirin.hawawa.org.

```
just install-landing
just landing-dev      # http://localhost:4321/
just landing-build    # static output in landingpage/dist/
just landing-check    # astro check + prettier
just landing-docker   # nginx image, tagged eirin-landingpage
```

- The page is `src/pages/index.astro`; `src/components/` has the screenshot and feature-row components.
- Screenshots are imported from `../docs/img/` (shared with the main README). Astro generates resized AVIF/WebP versions at build time, so commit full-resolution PNGs there.
- Clips in `../docs/vid/` (WebM) are imported with `?url` and rendered by `src/components/VideoShot.astro`, which shows a screenshot as the poster and only downloads and plays the clip once it scrolls into view.
- `site` and `base` default to `https://eirin.hawawa.org` and `/`. They're baked into canonical links, the og:image and the sitemap, so override them (`SITE_URL`, `BASE_PATH`, or the same-named Docker build args) when hosting elsewhere.

## Deployment

`.github/workflows/landingpage.yml` type-checks the site on PRs, and on pushes to `main` builds `landingpage/Dockerfile` (amd64 + arm64) and pushes it to `ghcr.io/hawawa4/eirin-landingpage` as `:latest` and `:sha-<commit>`. The image is nginx (unprivileged) serving the static build on port 8080; TLS, the domain and any reverse proxy are handled outside it.

```
docker run -d --name eirin-landingpage -p 8080:8080 ghcr.io/hawawa4/eirin-landingpage:latest
```

The Docker build context is the repo root (the media lives in `docs/`); `landingpage/Dockerfile.dockerignore` limits it to what the build needs.
