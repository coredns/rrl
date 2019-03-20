package cache

import (
	"github.com/pkg/errors"
	"hash/fnv"
	"sync"
)

// Hash returns the FNV hash of what.
func Hash(what []byte) uint64 {
	h := fnv.New64()
	h.Write(what)
	return h.Sum64()
}

// Cache is cache with a customizable eviction policy.
type Cache struct {
	shards [numShards]*shard
}

type EvictFn func(*interface{}) bool

// evictAll will evict the first item in the shard - effectively a random eviction
// this is the default mode of eviction if not set with SetEvict()
var evictAll = func(*interface{}) bool {return true}

// shard is a cache with customizable eviction policy.
type shard struct {
	items     map[string]*interface{}
	size      int
	evictable EvictFn

	sync.RWMutex
}

// New returns a new cache.
func New(size int) *Cache {
	ssize := size / numShards
	if ssize < 4 {
		ssize = 4
	}

	c := &Cache{}

	// Initialize all the shards
	for i := 0; i < numShards; i++ {
		c.shards[i] = newShard(ssize)
	}
	return c
}

func (c *Cache) SetEvict(e EvictFn) {
	for _, s := range c.shards {
		s.evictable = e
	}
}

func keyShard(key string) uint64 {
	return Hash([]byte(key)) & (numShards - 1)
}

// Add adds a new element to the cache. If the element already exists it is overwritten.
func (c *Cache) Add(key string, el interface{}) error {
	return c.shards[keyShard(key)].Add(key, el)
}

func (c *Cache) UpdateAdd(key string, update func(*interface{}) interface{}, add func() interface{}) interface{} {
	return c.shards[keyShard(key)].UpdateAdd(key, update, add)
}

// Get looks up element index under key.
func (c *Cache) Get(key string) (interface{}, bool) {
	return c.shards[keyShard(key)].Get(key)
}

// Remove removes the element indexed with key.
func (c *Cache) Remove(key string) {
	c.shards[keyShard(key)].Remove(key)
}

// Len returns an estimate number of elements in the cache.
// This is an estimate, because each shard is locked one at a time, and
// items can be added/removed from other shards as each shard is counted.
func (c *Cache) Len() int {
	l := 0
	for _, s := range c.shards {
		l += s.Len()
	}
	return l
}

// newShard returns a new shard with size.
func newShard(size int) *shard {
	return &shard{
		items: make(map[string]*interface{}),
		size: size,
		evictable: evictAll,
	}
}

// Add adds element indexed by key into the cache. Any existing element is overwritten
func (s *shard) Add(key string, el interface{}) error {
	s.Lock()
	if s.len() >= s.size && !s.evict() {
		s.Unlock()
		return errors.New("failed to add item, shard full")
	}

	s.items[key] = &el
	s.Unlock()
	return nil
}

// Remove locks the shard and removes the element indexed by key from the cache.
func (s *shard) Remove(key string) {
	s.Lock()
	s.remove(key)
	s.Unlock()
}

// remove removes the element indexed by key from the cache.
func (s *shard) remove(key string) {
	delete(s.items, key)
}

// evict removes the first evictable item from the shard.  If no items are evictable, return false.
func (s *shard) evict() bool {
	for key, item := range s.items {
		if !s.evictable(item) {
			continue
		}
		s.remove(key)
		return true
	}
	return false
}

// Get looks up the element indexed under key.
func (s *shard) Get(key string) (interface{}, bool) {
	s.RLock()
	el, found := s.items[key]
	s.RUnlock()
	if found {
		return *el, true
	}
	return nil, false
}

// UpdateAdd executes the function `update` on the element indexed under key.
// If key does not exist, then it is added, with a value equal to the result of function `add`.
func (s *shard) UpdateAdd(key string, update func(*interface{}) interface{}, add func() interface{}) interface{} {
	s.Lock()
	el, found := s.items[key]
	if found {
		resp := update(el)
		s.Unlock()
		return resp
	}
	l := len(s.items)
	if l >= s.size {
		if !s.evict() {
			return errors.New("failed to add item, shard full")
		}
	}
	newItem := add()
	s.items[key] = &newItem
	s.Unlock()
	return nil
}

// Len returns the current length of the cache.
func (s *shard) Len() int {
	s.RLock()
	l := s.len()
	s.RUnlock()
	return l
}

// len returns the current length of the cache.
func (s *shard) len() int {
	l := len(s.items)
	return l
}

const numShards = 256
