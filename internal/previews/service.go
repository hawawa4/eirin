package previews

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hawawa4/eirin/internal/fits"
)

// pipelineVersion is part of every cache key: bump it when rendering changes
// so stale images on disk are never served.
const pipelineVersion = 1

// jpegQuality for every level: visually lossless for astro images while
// keeping a 2048 px level around 1 MB.
const jpegQuality = 90

// renderSlots bounds concurrent renders: a native render of a large mosaic
// holds several hundred MB until it is encoded.
const renderSlots = 2

// warmShare is how much of the disk cache Warm may fill.
const warmShare = 0.8

// memCacheBytes bounds the in-memory LRU of encoded levels.
const memCacheBytes = 192 << 20

// Request names one preview.
type Request struct {
	Path string
	// Raster is true for PNG/JPEG/TIFF files, which are shown as they are.
	Raster bool
	// Stretch is the FITS stretch level: 0=linear … 3=strong.
	Stretch int
	// Size is the wanted longest side in pixels, snapped up to a level
	// (0 = native).
	Size int
}

// Image is an encoded preview level.
type Image struct {
	Data []byte
	// ETag identifies this response; it changes with the file.
	ETag string
	// ModTime is the source file's.
	ModTime time.Time
}

// Service renders and caches previews. Safe for concurrent use.
type Service struct {
	disk *diskCache // nil: memory only
	mem  memCache
	sem  chan struct{}

	mu    sync.Mutex
	jobs  map[string]*job
	metas map[string]meta

	userJobs atomic.Int32 // renders started by requests, not by Warm
	running  sync.WaitGroup // jobs, which outlive the request that started them
}

// New returns a Service caching on disk under dir (maxBytes total), or only
// in memory when dir is empty or unusable.
func New(dir string, maxBytes int64) *Service {
	s := &Service{
		mem:   memCache{maxBytes: memCacheBytes},
		sem:   make(chan struct{}, renderSlots),
		jobs:  map[string]*job{},
		metas: map[string]meta{},
	}
	if dir != "" {
		d, err := openDiskCache(dir, maxBytes)
		if err != nil {
			slog.Warn("previews: disk cache disabled", "dir", dir, "err", err)
		} else {
			s.disk = d
			slog.Info("previews: disk cache", "cache", d.String())
		}
	}
	return s
}

// source identifies the current version of r's file.
func source(r Request) (key string, mod time.Time, err error) {
	info, err := os.Stat(r.Path)
	if err != nil {
		return "", time.Time{}, err
	}
	if info.IsDir() {
		return "", time.Time{}, fmt.Errorf("%s is a directory", r.Path)
	}
	stretch := r.Stretch
	if r.Raster {
		stretch = -1
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%d\x00%s\x00%d\x00%d\x00%d",
		pipelineVersion, r.Path, info.Size(), info.ModTime().UnixNano(), stretch))
	return hex.EncodeToString(sum[:16]), info.ModTime(), nil
}

func etag(key string, level int) string {
	return fmt.Sprintf(`"%s-%d"`, key, level)
}

// Tag returns the ETag and modification time Get would answer r with, from
// the file's metadata alone (for conditional requests).
func (s *Service) Tag(r Request) (string, time.Time, error) {
	key, mod, err := source(r)
	if err != nil {
		return "", time.Time{}, err
	}
	return etag(key, SnapLevel(r.Size)), mod, nil
}

// Get returns the preview for r, rendering it when it isn't cached.
func (s *Service) Get(ctx context.Context, r Request) (Image, error) {
	key, mod, err := source(r)
	if err != nil {
		return Image{}, err
	}
	want := SnapLevel(r.Size)
	out := Image{ETag: etag(key, want), ModTime: mod}

	if m, ok := s.meta(key); ok {
		level := effectiveLevel(want, m.long())
		if b, ok := s.cached(key, level); ok {
			out.Data = b
			return out, nil
		}
	}
	b, err := s.await(ctx, s.startJob(key, r, true), want)
	if err != nil {
		return Image{}, err
	}
	out.Data = b
	return out, nil
}

// meta returns the native size recorded for key, from memory or disk.
func (s *Service) meta(key string) (meta, bool) {
	s.mu.Lock()
	m, ok := s.metas[key]
	s.mu.Unlock()
	if ok || s.disk == nil {
		return m, ok
	}
	if m, ok = s.disk.readMeta(key); ok {
		s.mu.Lock()
		s.metas[key] = m
		s.mu.Unlock()
	}
	return m, ok
}

func (s *Service) cached(key string, level int) ([]byte, bool) {
	if b, ok := s.mem.get(key, level); ok {
		return b, true
	}
	if s.disk == nil {
		return nil, false
	}
	b, ok := s.disk.read(key, level)
	if ok {
		s.mem.put(key, level, b)
	}
	return b, ok
}

// Warm renders every level of each request that isn't on disk yet, one at a
// time and only while no viewer is waiting for a render, until ctx is done.
// It stops early rather than fill the disk cache past warmShare of its
// budget, so warming never evicts what it (or a viewer) cached before.
func (s *Service) Warm(ctx context.Context, reqs []Request) {
	if s.disk == nil {
		return
	}
	done, failed := 0, 0
	for _, r := range reqs {
		if s.disk.fuller(warmShare) {
			slog.Info("previews: warm-up stopped, the cache is nearly full",
				"cache", s.disk.String(), "hint", "raise EIRIN_PREVIEW_CACHE_MB to keep more")
			break
		}
		for s.userJobs.Load() > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(250 * time.Millisecond):
			}
		}
		if ctx.Err() != nil {
			return
		}
		key, _, err := source(r)
		if err != nil || s.complete(key) {
			continue
		}
		j := s.startJob(key, r, false)
		select {
		case <-ctx.Done():
			return
		case <-j.done:
		}
		if j.err != nil {
			failed++
		} else {
			done++
		}
	}
	if done+failed > 0 {
		slog.Info("previews: warmed", "rendered", done, "failed", failed, "cache", s.disk.String())
	}
}

// complete reports whether every level of key is on disk.
func (s *Service) complete(key string) bool {
	m, ok := s.meta(key)
	if !ok {
		return false
	}
	for _, l := range levelsFor(m.long()) {
		if _, err := os.Stat(s.disk.path(key, levelFile(l))); err != nil {
			return false
		}
	}
	return true
}

// ── Render jobs ─────────────────────────────────────────────────────────────

// job renders one source version at every level. Requests for any of its
// levels wait on the same job.
type job struct {
	done chan struct{} // closed when the job ends

	mu      sync.Mutex
	notify  chan struct{} // closed and replaced whenever a level is ready
	long    int           // native longest side, once known
	results map[int][]byte
	wants   map[int]int // snapped level → waiters
	err     error
}

// startJob returns the running job for key, starting one if needed.
func (s *Service) startJob(key string, r Request, user bool) *job {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[key]; ok {
		return j
	}
	j := &job{
		done:    make(chan struct{}),
		notify:  make(chan struct{}),
		results: map[int][]byte{},
		wants:   map[int]int{},
	}
	s.jobs[key] = j
	if user {
		s.userJobs.Add(1)
	}
	s.running.Add(1)
	go s.run(key, r, j, user)
	return j
}

// await waits for level want of j.
func (s *Service) await(ctx context.Context, j *job, want int) ([]byte, error) {
	j.mu.Lock()
	j.wants[want]++
	defer func() {
		j.mu.Lock()
		j.wants[want]--
		j.mu.Unlock()
	}()
	for {
		level := effectiveLevel(want, j.long)
		if b, ok := j.results[level]; ok {
			j.mu.Unlock()
			return b, nil
		}
		select {
		case <-j.done:
			err := j.err
			j.mu.Unlock()
			if err == nil {
				err = fmt.Errorf("level %d was not rendered", level)
			}
			return nil, err
		default:
		}
		ch := j.notify
		j.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ch:
		case <-j.done:
		}
		j.mu.Lock()
	}
}

func (s *Service) run(key string, r Request, j *job, user bool) {
	if s.disk != nil {
		s.disk.pin(key)
	}
	defer func() {
		if p := recover(); p != nil {
			j.mu.Lock()
			j.err = fmt.Errorf("render %s: %v", filepath.Base(r.Path), p)
			j.mu.Unlock()
		}
		s.mu.Lock()
		delete(s.jobs, key)
		s.mu.Unlock()
		close(j.done)
		if user {
			s.userJobs.Add(-1)
		}
		if s.disk != nil {
			s.disk.unpin(key)
			s.disk.prune()
		}
		s.running.Done()
	}()

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	if err := s.render(key, r, j); err != nil {
		j.mu.Lock()
		j.err = err
		j.mu.Unlock()
	}
}

func (s *Service) render(key string, r Request, j *job) error {
	start := time.Now()
	var img image.Image
	var err error
	if r.Raster {
		img, err = fits.RenderRaster(r.Path, Native)
	} else {
		img, err = fits.RenderFITS(r.Path, Native, r.Stretch)
	}
	if err != nil {
		return err
	}
	b := img.Bounds()
	m := meta{Width: b.Dx(), Height: b.Dy()}

	// The whole pyramid is cheap next to the render: each level is shrunk
	// from the next larger one.
	levels := levelsFor(m.long())
	imgs := map[int]image.Image{Native: img}
	prev := img
	for _, l := range levels[:len(levels)-1] {
		prev = fits.Shrink(prev, l)
		imgs[l] = prev
	}

	s.mu.Lock()
	s.metas[key] = m
	s.mu.Unlock()
	if s.disk != nil {
		if err := s.disk.writeMeta(key, m); err != nil {
			slog.Warn("previews: write cache", "err", err)
		}
	}
	j.mu.Lock()
	j.long = m.long()
	j.mu.Unlock()

	// Levels are encoded in parallel: the small ones (what a first view asks
	// for) are ready in milliseconds, while the native one may take a while.
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	for _, level := range levels {
		wg.Go(func() {
			data, err := s.encode(imgs[level])
			if err != nil {
				errMu.Lock()
				firstErr = cmp.Or(firstErr, err)
				errMu.Unlock()
				return
			}
			if s.disk != nil {
				if err := s.disk.write(key, levelFile(level), data); err != nil {
					slog.Warn("previews: write cache", "err", err)
				}
			}
			j.mu.Lock()
			j.results[level] = data
			wanted := j.wanted(level)
			close(j.notify)
			j.notify = make(chan struct{})
			j.mu.Unlock()
			if wanted || s.disk == nil {
				s.mem.put(key, level, data)
			}
		})
	}
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	slog.Debug("previews: rendered", "path", r.Path, "size", fmt.Sprintf("%dx%d", m.Width, m.Height),
		"levels", len(levels), "took", time.Since(start))
	return nil
}

// wanted reports whether someone waits for level. Call with j.mu held.
func (j *job) wanted(level int) bool {
	for want, n := range j.wants {
		if n > 0 && effectiveLevel(want, j.long) == level {
			return true
		}
	}
	return false
}

// encode encodes one level. Every level is re-encoded, a JPEG original's
// native one too: decoded pixels carry no EXIF orientation, so all levels
// show the same pixels (the ones a plate solution refers to).
func (s *Service) encode(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, errors.New("no image for level")
	}
	return fits.EncodeJPEG(img, jpegQuality)
}
