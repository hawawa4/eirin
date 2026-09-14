# Eirin

Eirin (/ˈei̯rɪn/), a local multiplatform desktop app for managing astrophotography libraries. Built with Go + Wails + Svelte.


## Why this project exists

I have a Seestar S30Pro and S50, and processing the data in my computer (as opposed to the live stacking it does in the app) had a slightly awkward workflow consisting of multiple bash and python scripts to copy each session and then prepare the same session for Siril. Furthermore, inspecting all the FITS and deleting bad ones was a subpar experience: Siril won't (easily) let you completely delete files, ASIFitsView doesn't debayer the files and moving through different folders is painful.

The setup worked, but I wanted something a bit more user friendly, and, in general, *fun* to use, so I built this.

Eirin is aimed at helping you do all the data management, while the actual stacking and processing is done by a different software (namely, Siril, but it might support PixInsight in the future). 

## Expected Workflow

You designate a root folder (ideally in a NAS) to store *all* your astrophotography images, including subs, calibration frames (if applicable) and stacked/processed frames. Import from your Seestar and let Eirin catalogue everything! Then, once everything is tagged, you can create a *project* for stacking and processing frames. Once you're happy with the result, you reimport it to that library and then visualize it!


## Installing

### Linux

Grab the latest release from the [Releases page](https://github.com/hawawa4/eirin/releases). Three formats are published:

- **`.zip`** — extract and run the `eirin` binary directly
- **`.deb`** — `sudo apt install ./eirin-linux-amd64.deb` (Debian/Ubuntu and derivatives)
- **`.AppImage`** — `chmod +x eirin-linux-amd64.AppImage && ./eirin-linux-amd64.AppImage` (portable, works on most distros)

All three require `libwebkit2gtk-4.1-0` to be installed (the `.deb` declares this as a dependency automatically).

### Windows / macOS

Not yet published — build from source (see Development below).

# Features

## Browser View

A simple file browser view with preview of the FITS

[Browse View](docs/img/browseview.png)

## Library View
An enhanced file browser with additional grouping and filtering. This is the one you want to use most of the time
[Library View](docs/img/libraryview.png)

## Sky Atlas

Visualize your processed images in the sky!

[Sky Atlas](docs/img/skyatlasview.png)

## Projects

Create projects for Siril, preparing the folder structure that it expects
NOTE: Only Seestar projects (==only *lights* frames) supported for now

[Projects View](docs/img/projectview.png)

## Settings
Set your settings, scan the folder

[Settings](docs/img/settingsview.png)


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



# Development

## Prerequisites

| Tool | Tested version |
|------|---------------|
| Go | 1.25+ |
| Node.js | 22+ |
| Wails CLI | v3 (alpha) |
| staticcheck | v0.7.0+ |
| just | any recent |

Install Go tools once (`-tags gtk3` targets Linux's older webkit2gtk-4.1 stack — see the Linux/Ubuntu note below; omit it on macOS/Windows):
```
go install -tags gtk3 github.com/wailsapp/wails/v3/cmd/wails3@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
```

Install dependencies:
```
just install-backend
just install-frontend
```

## Running in dev mode

```
just dev
```

Wails launches a live-reload server: Go backend recompiles on save, Svelte/Vite handles frontend HMR. The app window opens automatically.

## Building a production binary

```
just build
```

Output is placed in `bin/`.

Other useful build recipes:
- `just build-backend` — Go backend only, no frontend/Wails packaging (`go build ./`)
- `just build-frontend` — Svelte frontend only (`npm run build` in `frontend/`)
- `just build-server` — headless server binary for Docker (`eirin-server`, see [Headless/server mode](#headlessserver-mode) below)
- `just check` — type-check only (`go build ./...` + `svelte-check`/`tsc`), no binaries produced
- `just lint` / `just format` — staticcheck + ESLint/Prettier

TODO: only Linux binaries are built and released right now (see [Installing](#installing)); Windows/macOS builds work locally via `wails3`/Task but aren't wired into CI yet.

## Technical details

### Linux / Ubuntu note

Wails requires a WebKit GTK library. Ubuntu 22.04+ and derivatives (including PikaOS) ship `libwebkit2gtk-4.1` rather than the older `4.0` variant:

```
sudo apt install libwebkit2gtk-4.1-dev
```

Wails v3 defaults to GTK4/webkitgtk-6.0 on Linux; this project targets the older GTK3/webkit2gtk-4.1 stack instead via the `gtk3` build tag, which `build/linux/Taskfile.yml` already passes to `go build`/`wails3 generate bindings` automatically. The `wails3` CLI itself also needs to be installed with that tag — `go install -tags gtk3 github.com/wailsapp/wails/v3/cmd/wails3@latest` — otherwise installing the CLI fails looking for GTK4 headers.


### Preferences database

Eirin stores metadata about the files in an SQLite database. By efault, it's stored in the platform config directory:

| Platform | Path |
|----------|------|
| Linux | `~/.config/eirin/prefs.db` |
| macOS | `~/Library/Application Support/eirin/prefs.db` |
| Windows | `%AppData%\eirin\prefs.db` |

But it can be overriden by setting the environment variable `EIRIN_DB_PATH`


### Localhost Port

Due to the `wails` architecture, Eirin will start a webserver on a local port. If that port is taken, you might have to set the environment variable `EIRIN_PORT`


### Headless/server mode

`just build-server` builds a `-tags server` binary intended for headless Docker deployment: read-only visualization (library browse, FITS preview, sky atlas, storage stats) with no import, no Siril processing, and no project management — those require the desktop app.

Pre-built images are published to `ghcr.io/hawawa4/eirin-server` on tagged releases:
```
docker run -p 8080:8080 -p 7070:7070 -v /path/to/nas:/mnt/nas -v /path/to/prefs.db:/root/.config/eirin/prefs.db ghcr.io/hawawa4/eirin-server:latest
```

Note the root folder path and everything else is stored in the prefs SQLite DB (no env var or flag sets it directly) — the DB is written by the desktop app's folder picker, so mount a `prefs.db` that already has `RootFolder` pointed at wherever you mount the NAS inside the container (or override the DB location with `EIRIN_DB_PATH`). There's currently no way to configure a from-scratch headless container without running the desktop app once first.

To build the image locally instead: `docker build -t eirin-server .` (`just docker-build` uses buildx's `local` output instead, extracting the static `eirin-server` binary to `./dist/` rather than producing a runnable image).

Two independent HTTP listeners run in the container: Wails' own server (serves the Svelte frontend, `WAILS_SERVER_PORT`, default 8080) and Eirin's side-channel REST API (`/api/status`, `/api/frames`, `/api/image`, via `EIRIN_PORT`, default 7070).


## AI Disclaimer
A significant amount of the code in this project was AI generated, I don't try to hide that fact: I can't write any respectable frontend code, but I'm half decent at backend and systems design. This project was also the first time I used `wails`

## Similar software

There's other wonderful astrophotgraphy software with some overlap in features, so you might be asking "why would I use this instead?" 

- [Astro Catalogue Viewer](https://github.com/thebioguy/Astro-Catalogue-Viewer): Genuinely great and fun to use software; does a similar thing of cataloguing images automatically, and the overall UX is reminiscent of a bird watching journal. It does not help with the processing step, but I can highly recommend it for viewing your finished images
- [SSLM](https://github.com/AstroNoob-Tools/SSLM): Windows only, focusing entirely on the Seestars