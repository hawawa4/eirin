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

# Type-check Go and TypeScript without producing binaries
check:
    go build ./...
    cd frontend && npm run check

# Build a production binary (handles frontend build internally)
build:
    wails build -tags webkit2_41

# ── Docker ───────────────────────────────────────────────────────────────────

# Build the Linux binary inside Docker; output written to ./dist/eirin
docker-build:
    DOCKER_BUILDKIT=1 docker build --output type=local,dest=dist .

# ── Linting & Formatting ─────────────────────────────────────────────────────

# Run all linters (Go staticcheck + frontend ESLint/Prettier)
lint: lint-go lint-frontend

# Run staticcheck on all Go packages
lint-go:
    staticcheck ./

# Run ESLint + Prettier check on the frontend
lint-frontend:
    cd frontend && npm run lint

# Format frontend source with Prettier
format:
    cd frontend && npm run format

# ── Utilities ────────────────────────────────────────────────────────────────

# Install frontend npm dependencies
install:
    cd frontend && npm install

# Remove all build artifacts
clean:
    rm -rf frontend/dist build/bin dist
