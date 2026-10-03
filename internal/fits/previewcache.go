package fits

import (
	"container/list"
	"fmt"
	"os"
	"sync"
)

// previewCacheBytes bounds the memory held by PreviewCache: ~400 Blink frames
// of a typical Seestar sub.
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
	key string
	url string
}

// Preview returns GeneratePreview(path, maxSize, stretchLevel), from the cache
// when the file is unchanged.
func (c *PreviewCache) Preview(path string, maxSize, stretchLevel int) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%d\x00%d", path, info.Size(), info.ModTime().UnixNano(), maxSize, stretchLevel)
	if url, ok := c.get(key); ok {
		return url, nil
	}
	url, err := GeneratePreview(path, maxSize, stretchLevel)
	if err != nil {
		return "", err
	}
	c.put(key, url)
	return url, nil
}

func (c *PreviewCache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return "", false
	}
	c.order.MoveToFront(el)
	return el.Value.(*previewEntry).url, true
}

func (c *PreviewCache) put(key, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = make(map[string]*list.Element)
	}
	if _, ok := c.items[key]; ok {
		return
	}
	c.items[key] = c.order.PushFront(&previewEntry{key: key, url: url})
	c.bytes += len(url)
	for c.bytes > previewCacheBytes && c.order.Len() > 1 {
		oldest := c.order.Back()
		e := oldest.Value.(*previewEntry)
		c.order.Remove(oldest)
		delete(c.items, e.key)
		c.bytes -= len(e.url)
	}
}
