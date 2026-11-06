# syntax=docker/dockerfile:1
# Static landing page served by nginx. Build from the repo root, since the
# screenshots and clips live in docs/:
#   docker build -f landingpage/Dockerfile -t eirin-landingpage .

# The output is plain HTML/CSS/media, so build on the native platform even for
# multi-arch images.
FROM --platform=$BUILDPLATFORM node:24-alpine AS build
WORKDIR /src/landingpage
COPY landingpage/package.json landingpage/package-lock.json ./
RUN npm ci
COPY docs/img /src/docs/img
COPY docs/vid /src/docs/vid
COPY landingpage/ ./
# Public URL baked into canonical links, the og:image and the sitemap.
ARG SITE_URL=https://eirin.hawawa.org
ARG BASE_PATH=/
# Optional: adds a "Photo gallery" link (the owner's eirin-server instance) to the page.
ARG GALLERY_URL=
RUN SITE_URL="$SITE_URL" BASE_PATH="$BASE_PATH" GALLERY_URL="$GALLERY_URL" npm run build

FROM nginxinc/nginx-unprivileged:1.31-alpine
COPY landingpage/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/landingpage/dist /usr/share/nginx/html
EXPOSE 8080
