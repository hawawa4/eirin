package previews

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"
)

// diskCache keeps rendered levels under dir/<key>/, one directory per source
// version (path, size, mtime, stretch): meta.json plus <level>.jpg files.
// Whole directories are evicted, least recently used first, once the total
// exceeds maxBytes.
type diskCache struct {
	dir      string
	maxBytes int64

	mu      sync.Mutex
	entries map[string]*diskEntry
	total   int64
	pinned  map[string]int // keys being rendered: never evicted
}

type diskEntry struct {
	bytes int64
	used  time.Time
}

// meta is what a cache directory records about its source image.
type meta struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (m meta) long() int { return max(m.Width, m.Height) }

// touchEvery limits how often a hit rewrites a directory's mtime (its LRU stamp).
const touchEvery = time.Hour

func openDiskCache(dir string, maxBytes int64) (*diskCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	c := &diskCache{dir: dir, maxBytes: maxBytes, entries: map[string]*diskEntry{}, pinned: map[string]int{}}
	dirs, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		info, err := d.Info()
		if err != nil {
			continue
		}
		e := &diskEntry{used: info.ModTime()}
		files, _ := os.ReadDir(filepath.Join(dir, d.Name()))
		for _, f := range files {
			if fi, err := f.Info(); err == nil {
				e.bytes += fi.Size()
			}
		}
		c.entries[d.Name()] = e
		c.total += e.bytes
	}
	c.prune()
	return c, nil
}

func levelFile(level int) string {
	if level == Native {
		return "native.jpg"
	}
	return strconv.Itoa(level) + ".jpg"
}

func (c *diskCache) path(key, name string) string {
	return filepath.Join(c.dir, key, name)
}

// readMeta returns the source's recorded size, if this version was rendered.
func (c *diskCache) readMeta(key string) (meta, bool) {
	b, err := os.ReadFile(c.path(key, "meta.json"))
	if err != nil {
		return meta{}, false
	}
	var m meta
	if json.Unmarshal(b, &m) != nil || m.Width <= 0 || m.Height <= 0 {
		return meta{}, false
	}
	return m, true
}

// read returns a cached level.
func (c *diskCache) read(key string, level int) ([]byte, bool) {
	b, err := os.ReadFile(c.path(key, levelFile(level)))
	if err != nil {
		return nil, false
	}
	c.touch(key)
	return b, true
}

func (c *diskCache) touch(key string) {
	c.mu.Lock()
	e := c.entries[key]
	stale := e != nil && time.Since(e.used) > touchEvery
	if stale {
		e.used = time.Now()
	}
	c.mu.Unlock()
	if stale {
		now := time.Now()
		_ = os.Chtimes(filepath.Join(c.dir, key), now, now)
	}
}

// write stores one file of key's directory atomically (temp file + rename).
func (c *diskCache) write(key, name string, data []byte) error {
	dir := filepath.Join(c.dir, key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, name)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	c.mu.Lock()
	e := c.entries[key]
	if e == nil {
		e = &diskEntry{}
		c.entries[key] = e
	}
	e.bytes += int64(len(data))
	e.used = time.Now()
	c.total += int64(len(data))
	c.mu.Unlock()
	return nil
}

func (c *diskCache) writeMeta(key string, m meta) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return c.write(key, "meta.json", b)
}

// pin keeps key's directory from being evicted until unpin.
func (c *diskCache) pin(key string) {
	c.mu.Lock()
	c.pinned[key]++
	c.mu.Unlock()
}

func (c *diskCache) unpin(key string) {
	c.mu.Lock()
	if c.pinned[key]--; c.pinned[key] <= 0 {
		delete(c.pinned, key)
	}
	c.mu.Unlock()
}

// fuller reports whether the cache holds more than share of its budget.
func (c *diskCache) fuller(share float64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxBytes > 0 && float64(c.total) > share*float64(c.maxBytes)
}

// prune removes least recently used directories until the cache is back
// under 90% of its budget.
func (c *diskCache) prune() {
	c.mu.Lock()
	if c.maxBytes <= 0 || c.total <= c.maxBytes {
		c.mu.Unlock()
		return
	}
	keys := make([]string, 0, len(c.entries))
	for k := range c.entries {
		if c.pinned[k] == 0 {
			keys = append(keys, k)
		}
	}
	slices.SortFunc(keys, func(a, b string) int { return c.entries[a].used.Compare(c.entries[b].used) })
	target := c.maxBytes * 9 / 10
	var victims []string
	for _, k := range keys {
		if c.total <= target {
			break
		}
		c.total -= c.entries[k].bytes
		delete(c.entries, k)
		victims = append(victims, k)
	}
	c.mu.Unlock()
	for _, k := range victims {
		if err := os.RemoveAll(filepath.Join(c.dir, k)); err != nil {
			slog.Warn("previews: evict", "key", k, "err", err)
		}
	}
}

func (c *diskCache) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf("%s (%d MB of %d MB)", c.dir, c.total>>20, c.maxBytes>>20)
}
