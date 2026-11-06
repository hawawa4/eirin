//go:build server

package app

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/hawawa4/eirin/internal/gallery"
	"github.com/hawawa4/eirin/internal/indexer"
	"github.com/hawawa4/eirin/internal/previews"
	"github.com/hawawa4/eirin/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// The server build is a public gallery, not a copy of the app. It binds this
// service to Wails *instead of* App, so everything App can do (library
// listings, file paths, settings, …) doesn't exist on the wire. Visitors only
// ever see GalleryInfo: final images under opaque ids (see gallery.PublicPath),
// never their real paths or file names.

// GalleryImage is one picture of the gallery, as visitors see it.
type GalleryImage struct {
	// Path is the opaque id the picture is requested by (PreviewRoute's path).
	Path string `json:"path"`
	// Title is the object's name.
	Title string `json:"title"`
	// Format is "fits" or "raster".
	Format string `json:"format"`
	gallery.Details

	// Plate solution; Solved is false when the picture has none (it can't be
	// placed on the Sky Atlas).
	Solved     bool    `json:"solved"`
	RA         float64 `json:"ra"`
	Dec        float64 `json:"dec"`
	PixelScale float64 `json:"pixelScale"` // arcsec/pixel
	Rotation   float64 `json:"rotation"`   // degrees, north through east

	// real is the library path; it never leaves the server.
	real string
}

// GalleryInfo is everything the gallery's page needs, in one reply.
type GalleryInfo struct {
	gallery.Config
	// Loaded is false until a library snapshot has been read.
	Loaded bool `json:"loaded"`
	// Images are the pictures of the Grid and the Slideshow.
	Images []GalleryImage `json:"images"`
	// Sky are the plate-solved pictures of the Sky Atlas: the Images that
	// have a solution, or — when none of them do (PNG/JPEG exports carry no
	// WCS) — the plate-solved FITS files.
	Sky []GalleryImage `json:"sky"`
}

// galleryState is the gallery built from one snapshot; immutable once built.
type galleryState struct {
	info  GalleryInfo
	index *gallery.Index
	// warm lists what to pre-render: every picture a visitor can open.
	warm []previews.Request
}

// gallerySide is the server's gallery: its configuration and the state of the
// snapshot loaded last.
type gallerySide struct {
	cfg   gallery.Config
	state atomic.Pointer[galleryState]
}

// loadGalleryConfig reads the gallery's configuration; the JSON file defaults
// to gallery.json next to the server's database (the /data volume).
func (a *App) loadGalleryConfig(dbPath string) error {
	cfg, err := gallery.Load(os.Getenv, filepath.Join(filepath.Dir(dbPath), "gallery.json"))
	if err != nil {
		return err
	}
	a.snapshots.gallery.cfg = cfg
	slog.Info("gallery: configured", "title", cfg.Title, "view", cfg.View, "format", cfg.Format, "theme", cfg.Theme)
	return nil
}

// refreshGallery rebuilds the gallery from a newly loaded snapshot.
func (a *App) refreshGallery(st *store.Store) {
	frames, err := st.GetAllFramesUnder(st.Load().RootFolder)
	if err != nil {
		slog.Error("gallery: reading the snapshot", "err", err)
		return
	}
	a.snapshots.gallery.state.Store(buildGallery(a.snapshots.gallery.cfg, frames))
}

// buildGallery picks the pictures to show out of a snapshot's frames.
func buildGallery(cfg gallery.Config, frames []store.Frame) *galleryState {
	var fits, raster []store.Frame
	for _, f := range frames {
		if f.Rejected || (f.FrameType != store.FrameTypeProcessed && f.FrameType != store.FrameTypeImage) {
			continue
		}
		if indexer.IsRasterFile(f.NasPath) {
			raster = append(raster, f)
		} else if indexer.IsFitsFile(f.NasPath) {
			fits = append(fits, f)
		}
	}
	format := gallery.ChooseFormat(cfg.Format, len(fits), len(raster))
	shown, other := fits, raster
	if format == gallery.FormatRaster {
		shown, other = raster, fits
	}

	images := toGalleryImages(shown)
	sky := solved(images)
	if len(sky) == 0 {
		sky = solved(toGalleryImages(other))
	}

	// Every file a visitor can reach is in the index (and nothing else is).
	var reals []string
	var warm []previews.Request
	seen := map[string]bool{}
	for _, list := range [][]GalleryImage{images, sky} {
		for _, img := range list {
			if seen[img.real] {
				continue
			}
			seen[img.real] = true
			reals = append(reals, img.real)
			// Finished pictures are shown as they are: stretch 0, as in
			// stretchFor (frontend/src/lib/images.ts).
			warm = append(warm, previews.Request{Path: img.real, Raster: indexer.IsRasterFile(img.real)})
		}
	}

	return &galleryState{
		info:  GalleryInfo{Config: cfg, Loaded: true, Images: images, Sky: sky},
		index: gallery.NewIndex(reals),
		warm:  warm,
	}
}

func toGalleryImages(frames []store.Frame) []GalleryImage {
	out := make([]GalleryImage, 0, len(frames))
	for _, f := range frames {
		img := GalleryImage{
			real:       f.NasPath,
			Path:       gallery.PublicPath(f.NasPath),
			Title:      strings.TrimSpace(f.Object),
			Format:     gallery.FormatFITS,
			Details:    gallery.Describe(filepath.Base(f.NasPath), f.Telescope, f.Filter, f.DateObs, f.ExpTime),
			RA:         derefFloat(f.RA),
			Dec:        derefFloat(f.Dec),
			PixelScale: derefFloat(f.PixelScale),
			Rotation:   derefFloat(f.Rotation),
		}
		if img.Title == "" {
			img.Title = "Untitled"
		}
		if indexer.IsRasterFile(f.NasPath) {
			img.Format = gallery.FormatRaster
		}
		img.Solved = img.RA != 0 && img.PixelScale != 0
		out = append(out, img)
	}
	slices.SortFunc(out, func(a, b GalleryImage) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)),
			strings.Compare(a.Date, b.Date),
			strings.Compare(a.Path, b.Path),
		)
	})
	return out
}

func solved(images []GalleryImage) []GalleryImage {
	out := make([]GalleryImage, 0, len(images))
	for _, img := range images {
		if img.Solved {
			out = append(out, img)
		}
	}
	return out
}

// publicToReal maps an opaque id to the library file behind it. Real paths
// are refused: visitors only know ids.
func (a *App) publicToReal(public string) (string, bool) {
	st := a.snapshots.gallery.state.Load()
	if st == nil {
		return "", false
	}
	return st.index.Real(public)
}

// Gallery is the one service the server build binds to Wails.
type Gallery struct {
	app *App
}

// NewGallery returns the gallery service of a.
func NewGallery(a *App) *Gallery { return &Gallery{app: a} }

// ServiceStartup and ServiceShutdown run the app's own lifecycle: Wails only
// calls them on the services it was given.
func (g *Gallery) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	return g.app.ServiceStartup(ctx, options)
}

func (g *Gallery) ServiceShutdown() error { return g.app.ServiceShutdown() }

// GetGallery returns the gallery: its title and texts, and its pictures.
func (g *Gallery) GetGallery() GalleryInfo {
	if st := g.app.snapshots.gallery.state.Load(); st != nil {
		return st.info
	}
	return GalleryInfo{Config: g.app.snapshots.gallery.cfg}
}

// GetCatalog returns the star catalog for the Sky Atlas's labels.
func (g *Gallery) GetCatalog() []CatalogObject { return g.app.GetCatalog() }

// GetImageSizes returns the pixel size of pictures by opaque id (the Sky
// Atlas needs it to outline each picture on the sky). Unknown ids are left out.
func (g *Gallery) GetImageSizes(paths []string) map[string]AtlasFrameSize {
	publicOf := make(map[string]string, len(paths)) // real → public
	reals := make([]string, 0, len(paths))
	for _, p := range paths {
		if real, ok := g.app.publicToReal(p); ok {
			publicOf[real] = p
			reals = append(reals, real)
		}
	}
	out := make(map[string]AtlasFrameSize, len(reals))
	for real, sz := range g.app.GetAtlasFrameSizes(reals) {
		out[publicOf[real]] = sz
	}
	return out
}
