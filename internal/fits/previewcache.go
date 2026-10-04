package fits

import (
	"container/list"
	"fmt"
	"os"
	"sync"
)

// previewCacheBytes bounds the memory held by PreviewCache: ~400 Blink frames
// of a typical Seestar sub, or ~150 full-size viewer previews.
const previewCacheBytes = 128 << 20

// PreviewCache keeps recently generated preview data URLs in memory so
// blinking through the same frames again doesn't re-read them from the NAS.
// Entries are keyed by the file's size and modification time as well, so a
// changed file is regenerated. The zero value is ready to use.
type PreviewCache struct {
	mu    sync.Mutex
	items map[string]*list.Element
	order list.List // front = most recently used
	bytes int
}

type previewEntry struct {
	key     string
	preview RenderedPreview
}

// Preview returns GeneratePreview(path, maxSize, stretchLevel), from the cache
// when the file is unchanged.
func (c *PreviewCache) Preview(path string, maxSize, stretchLevel int) (string, error) {
	p, err := c.Rendered(path, maxSize, stretchLevel)
	return p.DataURL, err
}

// Rendered returns RenderPreview(path, maxSize, stretchLevel), from the cache
// when the file is unchanged.
func (c *PreviewCache) Rendered(path string, maxSize, stretchLevel int) (RenderedPreview, error) {
	return c.cached(path, fmt.Sprintf("fits\x00%d\x00%d", maxSize, stretchLevel), func() (RenderedPreview, error) {
		return RenderPreview(path, maxSize, stretchLevel)
	})
}

// Raster returns RenderRasterPreview(path, maxSize), from the cache when the
// file is unchanged.
func (c *PreviewCache) Raster(path string, maxSize int) (RenderedPreview, error) {
	return c.cached(path, fmt.Sprintf("raster\x00%d", maxSize), func() (RenderedPreview, error) {
		return RenderRasterPreview(path, maxSize)
	})
}

func (c *PreviewCache) cached(path, variant string, render func() (RenderedPreview, error)) (RenderedPreview, error) {
	info, err := os.Stat(path)
	if err != nil {
		return RenderedPreview{}, err
	}
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%s", path, info.Size(), info.ModTime().UnixNano(), variant)
	if p, ok := c.get(key); ok {
		return p, nil
	}
	p, err := render()
	if err != nil {
		return RenderedPreview{}, err
	}
	c.put(key, p)
	return p, nil
}

func (c *PreviewCache) get(key string) (RenderedPreview, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return RenderedPreview{}, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*previewEntry).preview, true
}

func (c *PreviewCache) put(key string, p RenderedPreview) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = make(map[string]*list.Element)
	}
	if _, ok := c.items[key]; ok {
		return
	}
	c.items[key] = c.order.PushFront(&previewEntry{key: key, preview: p})
	c.bytes += len(p.DataURL)
	for c.bytes > previewCacheBytes && c.order.Len() > 1 {
		oldest := c.order.Back()
		e := oldest.Value.(*previewEntry)
		c.order.Remove(oldest)
		delete(c.items, e.key)
		c.bytes -= len(e.preview.DataURL)
	}
}
