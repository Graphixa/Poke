package sprite

import "testing"

func TestMemCacheByteBudgetEvictsOldest(t *testing.T) {
	c := newANSIMemCacheBytes(10)
	c.Put("a", "aaaa") // 4
	c.Put("b", "bbbb") // 8
	c.Put("c", "cccc") // 12 > 10, evict a
	if c.len() != 2 {
		t.Fatalf("len=%d want 2", c.len())
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("oldest should be evicted")
	}
	if v, ok := c.Get("c"); !ok || v != "cccc" {
		t.Fatal("newest must stay")
	}
	if c.byteSize() > 10 {
		t.Fatalf("bytes=%d want <=10", c.byteSize())
	}
}

func TestMemCacheGetKeepsEntryHot(t *testing.T) {
	c := newANSIMemCacheBytes(8)
	c.Put("a", "aaaa")
	c.Put("b", "bbbb")
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a missing")
	}
	c.Put("c", "cccc")
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted after a was touched")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a should stay hot")
	}
}

func TestMemCacheSingleFrameMayExceedBudget(t *testing.T) {
	c := newANSIMemCacheBytes(4)
	c.Put("big", "0123456789")
	if c.len() != 1 {
		t.Fatalf("len=%d want 1 oversized frame", c.len())
	}
	if v, ok := c.Get("big"); !ok || v != "0123456789" {
		t.Fatal("oversized frame must still be stored")
	}
}

func TestMemCacheInvalidatePrefixDropsBytes(t *testing.T) {
	c := newANSIMemCacheBytes(100)
	c.Put("named-1-x", "aaaa")
	c.Put("named-2-y", "bbbb")
	c.InvalidatePrefix("named-1-")
	if _, ok := c.Get("named-1-x"); ok {
		t.Fatal("invalidated key still present")
	}
	if c.byteSize() != 4 {
		t.Fatalf("bytes=%d want 4", c.byteSize())
	}
}

func TestDefaultMemCacheBudget(t *testing.T) {
	if defaultMemCacheBytes < 4<<20 || defaultMemCacheBytes > 8<<20 {
		t.Fatalf("budget=%d want 4–8 MiB", defaultMemCacheBytes)
	}
}

func TestActivityGateThrottle(t *testing.T) {
	var g ActivityGate
	if g.ThrottleBackground() {
		t.Fatal("no activity should not throttle")
	}
	g.SetForegroundBusy(true)
	if !g.ThrottleBackground() {
		t.Fatal("foreground busy must throttle")
	}
	g.SetForegroundBusy(false)
	g.NoteActivity()
	if !g.ThrottleBackground() {
		t.Fatal("recent activity must throttle")
	}
}
