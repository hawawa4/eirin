# Headless/server build for Docker deployment: a public photo gallery (a grid
# of your finished images, the Sky Atlas and a Slideshow). It reads the library
# snapshot the desktop app publishes to <library>/.eirin/library.db (enable it
# in the desktop app's Settings → Server viewer) and never writes to the
# library; visitors never see the library itself, only the final images, by
# opaque ids. Mount the library at EIRIN_ROOT (read-only is fine) and /data for
# the server's local copy of the snapshot, its preview cache (/data/previews)
# and the optional gallery.json. Title, texts and theme come from
# EIRIN_GALLERY_* variables or /data/gallery.json (see the README).

FROM node:24-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build:gallery

FROM golang:1.26-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist-gallery ./frontend/dist-gallery
RUN CGO_ENABLED=0 go build -tags server -ldflags="-s -w" -o /out/eirin-server .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=backend-builder /out/eirin-server /usr/local/bin/eirin-server

# One HTTP listener (Wails' own server: the gallery page + its RPC/WebSocket
# events), port via WAILS_SERVER_PORT. The REST API is off in this build.
ENV WAILS_SERVER_HOST=0.0.0.0
ENV WAILS_SERVER_PORT=8080
ENV EIRIN_ROOT=/library
ENV EIRIN_DB_PATH=/data/library.db
VOLUME /data
EXPOSE 8080

ENTRYPOINT ["eirin-server"]
