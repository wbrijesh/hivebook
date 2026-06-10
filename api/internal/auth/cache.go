package auth

import (
	"container/list"
	"sync"
	"time"
)

// identityCache is a bounded, TTL'd cache of resolved identities keyed by user
// (sub). Bounded so a long-lived process can't grow without limit; TTL'd so a
// user who moves organizations is re-resolved rather than pinned to a stale
// tenant. Eviction is least-recently-used once the size cap is hit.
type identityCache struct {
	mu  sync.Mutex
	max int
	ttl time.Duration
	ll  *list.List               // MRU at the front, LRU at the back
	m   map[string]*list.Element // sub -> element in ll
}

type cacheEntry struct {
	sub     string
	id      Identity
	expires time.Time
}

func newIdentityCache(max int, ttl time.Duration) *identityCache {
	return &identityCache{
		max: max,
		ttl: ttl,
		ll:  list.New(),
		m:   make(map[string]*list.Element),
	}
}

// get returns the cached identity for sub, or false if absent or expired.
// An expired entry is dropped on read.
func (c *identityCache) get(sub string) (Identity, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.m[sub]
	if !ok {
		return Identity{}, false
	}
	ent := el.Value.(*cacheEntry)
	if time.Now().After(ent.expires) {
		c.remove(el)
		return Identity{}, false
	}
	c.ll.MoveToFront(el)
	return ent.id, true
}

// add inserts or refreshes the identity for sub and evicts the least-recently-
// used entry if the cache is over capacity.
func (c *identityCache) add(sub string, id Identity) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.m[sub]; ok {
		ent := el.Value.(*cacheEntry)
		ent.id = id
		ent.expires = time.Now().Add(c.ttl)
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(&cacheEntry{sub: sub, id: id, expires: time.Now().Add(c.ttl)})
	c.m[sub] = el
	if c.ll.Len() > c.max {
		c.remove(c.ll.Back())
	}
}

// remove unlinks el from both the list and the index. Caller holds c.mu.
func (c *identityCache) remove(el *list.Element) {
	if el == nil {
		return
	}
	c.ll.Remove(el)
	delete(c.m, el.Value.(*cacheEntry).sub)
}
