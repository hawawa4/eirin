package previews

import (
	"container/list"
	"sync"
)

// memCache is a byte-bounded LRU of encoded levels in front of the disk.
type memCache struct {
	maxBytes int

	mu    sync.Mutex
	items map[memKey]*list.Element
	order list.List // front = most recently used
	bytes int
}

type memKey struct {
	key   string
	level int
}

type memEntry struct {
	k    memKey
	data []byte
}

func (c *memCache) get(key string, level int) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[memKey{key, level}]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*memEntry).data, true
}

func (c *memCache) put(key string, level int, data []byte) {
	if len(data) > c.maxBytes/4 {
		return // a huge native image would flush everything else
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = make(map[memKey]*list.Element)
	}
	k := memKey{key, level}
	if _, ok := c.items[k]; ok {
		return
	}
	c.items[k] = c.order.PushFront(&memEntry{k: k, data: data})
	c.bytes += len(data)
	for c.bytes > c.maxBytes && c.order.Len() > 1 {
		oldest := c.order.Back()
		e := oldest.Value.(*memEntry)
		c.order.Remove(oldest)
		delete(c.items, e.k)
		c.bytes -= len(e.data)
	}
}
