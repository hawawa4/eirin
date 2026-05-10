# Eirin

A local, multiplatform desktop app for managing astrophotography libraries. Built with Go + Wails + Svelte.

## What it does

Eirin sits between your NAS and your image-processing workflow. It lets you browse your raw astrophotography files, inspect FITS headers, and preview autostretched images — without uploading anything to a cloud service or running a web server.

## Features

### File browser
- Select any local or NAS-mounted root folder
- Recursive directory navigation with back-navigation history
- Resizable left pane; collapse it entirely to maximise the preview area
- Sorted file listing with name, modified date, and size

### FITS preview
- Click any `.fit` / `.fits` file to generate an autostretched PNG preview
- Zoom with the scroll wheel (centered on cursor position); drag to pan; double-click to reset
- **Stretch toggle** — switch between linear and non-linear display:
  - **Off (linear)** — shadow-clipped linear rescale; raw-looking image with a dark sky
  - **Gentle** — mild MTF autostretch (−1.25σ shadow clip, background target 10%)
  - **Normal** — Siril-default MTF stretch (−2.80σ, background target 25%)
  - **Strong** — aggressive MTF stretch (−4.00σ, background target 40%)
- Correct handling of Bayer-mosaic single-plane FITS (Seestar and similar cameras): demosaiced to RGB via 2×2 block averaging before preview

### FITS header panel
- **Basic** section: Object, Filter, Exposure, Date, Image size
- **Advanced** section: Gain, CCD temperature, Telescope, Camera, Binning
- Both sections collapse/expand independently

### Preferences
All UI state is persisted automatically to a local SQLite database — no explicit "Save" button needed. Restored on next launch:
- Root folder
- Stretch toggle and aggressiveness preset
- Basic / Advanced header section collapse state

## Getting started

Select **Select Root Folder** and point Eirin at your NAS mount or local astrophotography directory. Click any FITS file to preview it. Adjust the stretch preset until the image looks the way you want.

## Development

### Prerequisites

| Tool | Tested version |
|------|---------------|
| Go | 1.25+ |
| Node.js | 22+ |
| Wails CLI | v2.12.0 |
| staticcheck | v0.7.0+ |
| just | any recent |

Install Go tools once:
```
go install github.com/wailsapp/wails/v2/cmd/wails@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

Install frontend dependencies:
```
just install
```

### Running in dev mode

```
just dev
```

Wails launches a live-reload server: Go backend recompiles on save, Svelte/Vite handles frontend HMR. The app window opens automatically.

### Building a production binary

```
just build
```

Output is placed in `build/bin/`.

### Building inside Docker (headless / CI)

```
just docker-build
```

Produces a statically-linked Linux binary at `dist/eirin`.

### All Justfile commands

```
just dev             # live-reload dev mode
just build           # production binary
just build-frontend  # Svelte only (fast, for UI-only iteration)
just build-backend   # Go only
just check           # type-check Go + TypeScript without producing binaries
just lint            # run all linters (staticcheck + ESLint/Prettier)
just lint-go         # staticcheck on Go source
just lint-frontend   # ESLint + Prettier check on frontend source
just format          # format frontend source with Prettier
just install         # npm install for the frontend
just clean           # remove all build artefacts
just docker-build    # build via Docker
```

## Project structure

```
eirin/
├── main.go                  # Wails entry point (embed constraint keeps it here)
├── internal/
│   ├── app/app.go           # Wails-bound methods (thin adapter layer)
│   ├── browser/browser.go   # Directory listing
│   ├── fits/fits.go         # FITS header reading, debayer, autostretch, PNG preview
│   └── prefs/prefs.go       # SQLite preference store (squirrel query builder)
├── frontend/
│   └── src/App.svelte       # Single-page Svelte UI
├── Justfile
└── Dockerfile
```

## Linux / Ubuntu note

Wails requires a WebKit GTK library. Ubuntu 22.04+ and derivatives (including PikaOS) ship `libwebkit2gtk-4.1` rather than the older `4.0` variant:

```
sudo apt install libwebkit2gtk-4.1-dev
```

The Justfile already passes `-tags webkit2_41` to all Wails commands so this is handled automatically.

## Preferences database

The SQLite database is stored in the platform config directory:

| Platform | Path |
|----------|------|
| Linux | `~/.config/eirin/prefs.db` |
| macOS | `~/Library/Application Support/eirin/prefs.db` |
| Windows | `%AppData%\eirin\prefs.db` |

## Planned features

- Copy from Seestar to NAS via rsync (no duplicate files)
- Automatic sequence creation and Siril CLI integration
- Symlink-based local project management for NAS-resident data
- Batch delete / move from the browser
