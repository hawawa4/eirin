# Ensure Go tools and Wails are on PATH for every recipe
export PATH := env_var('HOME') + "/go/bin:/usr/local/go/bin:" + env_var('PATH')

# List available recipes
default:
    @just --list

# ── Development ──────────────────────────────────────────────────────────────

# Live-reload dev mode: Wails rebuilds Go on source change, Vite handles Svelte HMR
dev:
    wails dev -tags webkit2_41 # Ubuntu/Debian uses this version

# ── Build ─────────────────────────────────────────────────────────────────────

# Build only the Svelte frontend into frontend/dist/
build-frontend:
    cd frontend && npm run build

# Build only the Backend
build-backend:
    go build ./

# Run Go unit tests
test:
    go test ./internal/...

# Type-check Go and TypeScript without producing binaries
check:
    go build ./...
    cd frontend && npm run check

# Build a production binary — Linux Debian/Ubuntu (webkit2_41)
build:
    wails build -tags webkit2_41

# Build for Linux generic (no special webkit tag)
build-linux:
    wails build -platform linux/amd64

# Build for Windows (cross-compile via Wails)
build-windows:
    wails build -platform windows/amd64

# Build for macOS Apple Silicon
build-macos-arm:
    wails build -platform darwin/arm64

# ── Docker ───────────────────────────────────────────────────────────────────

# Build the Linux binary inside Docker; output written to ./dist/eirin
docker-build:
    DOCKER_BUILDKIT=1 docker build --output type=local,dest=dist .

# ── Linting & Formatting ─────────────────────────────────────────────────────

# Run all linters (Go staticcheck + frontend ESLint/Prettier)
lint: lint-go lint-frontend

# Run staticcheck on all Go packages
lint-go:
    staticcheck ./...

# Run ESLint + Prettier check on the frontend
lint-frontend:
    cd frontend && npm run lint

# Format frontend source with Prettier
format:
    cd frontend && npm run format

# ── Utilities ────────────────────────────────────────────────────────────────

# Install frontend npm dependencies
install-frontend:
    cd frontend && npm install

install-backend:
    go mod tidy

# Remove all build artifacts
clean:
    rm -rf frontend/dist build/bin dist
