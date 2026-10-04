# Headless/server build for Docker deployment: a read-only viewer of the final
# images (stacked/processed/raster) and the Sky Atlas. It reads the library
# snapshot the desktop app publishes to <library>/.eirin/library.db (enable it
# in the desktop app's Settings → Server viewer) and never writes to the
# library. Mount the library at EIRIN_ROOT (read-only is fine) and /data for
# the server's local copy of the snapshot.

FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -tags server -ldflags="-s -w" -o /out/eirin-server .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=backend-builder /out/eirin-server /usr/local/bin/eirin-server

# Two independent HTTP listeners run in this process:
#   - Wails' own server (serves the Svelte UI + RPC/WebSocket events), port via WAILS_SERVER_PORT
#   - Eirin's side-channel REST API (/api/status, /api/frames, /api/image), port via EIRIN_PORT
ENV WAILS_SERVER_HOST=0.0.0.0
ENV WAILS_SERVER_PORT=8080
ENV EIRIN_PORT=7070
ENV EIRIN_ROOT=/library
ENV EIRIN_DB_PATH=/data/library.db
VOLUME /data
EXPOSE 8080 7070

ENTRYPOINT ["eirin-server"]
