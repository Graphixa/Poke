package sprite

import "sync"

// ansiMemCache is a tiny process-local LRU for rendered ANSI frames so
// selection changes that re-hit the same key skip disk and Chafa work.
type ansiMemCache struct {
	mu    sync.Mutex
	order []string
	items map[string]string
	cap   int
}

func newANSIMemCache(capacity int) *ansiMemCache {
	if capacity < 1 {
		capacity = 64
	}
	return &ansiMemCache{
		items: make(map[string]string, capacity),
		cap:   capacity,
	}
}

func (c *ansiMemCache) Get(key string) (string, bool) {
	if c == nil || key == "" {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.items[key]
	if !ok {
		return "", false
	}
	// Refresh LRU position.
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
	return v, true
}

func (c *ansiMemCache) Put(key, ansi string) {
	if c == nil || key == "" || ansi == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; ok {
		c.items[key] = ansi
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
		c.order = append(c.order, key)
		return
	}
	for len(c.order) >= c.cap {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
	c.items[key] = ansi
	c.order = append(c.order, key)
}
