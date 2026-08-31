package sprite

import (
	"strings"
	"sync"
)

// defaultMemCacheBytes is the process-local rendered-frame budget (6 MiB).
// Disk artefact cache remains the durable store for older frames.
const defaultMemCacheBytes = 6 << 20

// defaultNativeMemCacheBytes holds decompressed sixel/native frames (4 MiB).
const defaultNativeMemCacheBytes = 4 << 20

// defaultAnsiMemCacheBytes holds chafa/halfblock ANSI frames (2 MiB).
const defaultAnsiMemCacheBytes = 2 << 20

// frameMemCache is a process-local byte-budgeted LRU for rendered frame text.
type frameMemCache struct {
	mu       sync.Mutex
	order    []string
	items    map[string]string
	bytes    int
	maxBytes int
}

func newFrameMemCacheBytes(maxBytes int) *frameMemCache {
	if maxBytes < 1 {
		maxBytes = defaultMemCacheBytes
	}
	return &frameMemCache{
		items:    make(map[string]string),
		maxBytes: maxBytes,
	}
}

func (c *frameMemCache) Get(key string) (string, bool) {
	if c == nil || key == "" {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[key]
	if !ok {
		return "", false
	}
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
	return v, true
}

func (c *frameMemCache) Put(key, ansi string) {
	if c == nil || key == "" || ansi == "" {
		return
	}
	n := len(ansi)
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.items[key]; ok {
		c.bytes -= len(old)
		c.items[key] = ansi
		c.bytes += n
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
		c.order = append(c.order, key)
		c.evictLocked()
		return
	}
	c.items[key] = ansi
	c.bytes += n
	c.order = append(c.order, key)
	c.evictLocked()
}

func (c *frameMemCache) evictLocked() {
	for c.bytes > c.maxBytes && len(c.order) > 1 {
		oldest := c.order[0]
		c.order = c.order[1:]
		if v, ok := c.items[oldest]; ok {
			c.bytes -= len(v)
			delete(c.items, oldest)
		}
	}
}

func (c *frameMemCache) byteSize() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}

func (c *frameMemCache) len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// InvalidatePrefix removes all keys that contain substr (species named keys).
func (c *frameMemCache) InvalidatePrefix(substr string) {
	if c == nil || substr == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	kept := c.order[:0]
	for _, k := range c.order {
		if strings.Contains(k, substr) {
			c.bytes -= len(c.items[k])
			delete(c.items, k)
			continue
		}
		kept = append(kept, k)
	}
	c.order = kept
}
