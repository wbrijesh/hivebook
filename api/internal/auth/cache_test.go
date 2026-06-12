package auth

import (
	"sync"
	"testing"
	"time"
)

func TestIdentityCache_MissThenHit(t *testing.T) {
	c := newIdentityCache(4, time.Minute)

	if _, ok := c.get("u1"); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.add("u1", Identity{Sub: "u1", Org: "o1"})
	got, ok := c.get("u1")
	if !ok || got.Org != "o1" {
		t.Fatalf("expected hit with org o1, got %+v ok=%v", got, ok)
	}
}

func TestIdentityCache_LRUEviction(t *testing.T) {
	c := newIdentityCache(2, time.Minute)
	c.add("a", Identity{Sub: "a"})
	c.add("b", Identity{Sub: "b"})

	// Touch "a" so "b" is now least-recently-used.
	if _, ok := c.get("a"); !ok {
		t.Fatal("a should be present")
	}
	// Adding a third entry evicts the LRU ("b"), keeps "a" and "c".
	c.add("c", Identity{Sub: "c"})

	if _, ok := c.get("b"); ok {
		t.Fatal("b should have been evicted as least-recently-used")
	}
	if _, ok := c.get("a"); !ok {
		t.Fatal("a should survive (recently used)")
	}
	if _, ok := c.get("c"); !ok {
		t.Fatal("c should be present (just added)")
	}
}

func TestIdentityCache_TTLExpiry(t *testing.T) {
	now := time.Unix(0, 0)
	c := newIdentityCache(4, 30*time.Minute)
	c.now = func() time.Time { return now }

	c.add("u1", Identity{Sub: "u1"})

	now = now.Add(29 * time.Minute)
	if _, ok := c.get("u1"); !ok {
		t.Fatal("entry should still be live before TTL")
	}

	now = now.Add(2 * time.Minute) // 31m total > 30m TTL
	if _, ok := c.get("u1"); ok {
		t.Fatal("entry should be expired after TTL")
	}
	// Expired entry is dropped from the index, not just hidden.
	if _, ok := c.m["u1"]; ok {
		t.Fatal("expired entry should be removed on read")
	}
}

func TestIdentityCache_RefreshKeepsOneEntry(t *testing.T) {
	c := newIdentityCache(4, time.Minute)
	c.add("u1", Identity{Sub: "u1", Org: "o1"})
	c.add("u1", Identity{Sub: "u1", Org: "o2"}) // re-add updates in place

	if c.ll.Len() != 1 {
		t.Fatalf("expected 1 entry after re-add, got %d", c.ll.Len())
	}
	got, _ := c.get("u1")
	if got.Org != "o2" {
		t.Fatalf("expected refreshed org o2, got %s", got.Org)
	}
}

// Exercises the mutex under -race.
func TestIdentityCache_Concurrent(t *testing.T) {
	c := newIdentityCache(64, time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sub := string(rune('a' + i%26))
			c.add(sub, Identity{Sub: sub})
			c.get(sub)
		}(i)
	}
	wg.Wait()
}
