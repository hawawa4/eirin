# ── Stage 1: Build ───────────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS builder

# Install Node.js 22
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Wails Linux build dependencies (GTK3 + WebKit2GTK embedded browser)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libgtk-3-dev \
    libwebkit2gtk-4.0-dev \
    && rm -rf /var/lib/apt/lists/*

# Install the Wails CLI matching the project version
RUN go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

WORKDIR /app

# Resolve Go modules first so this layer is cached between source changes
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN wails build

# ── Stage 2: Export binary ────────────────────────────────────────────────────
FROM scratch AS export
COPY --from=builder /app/build/bin/eirin /eirin
