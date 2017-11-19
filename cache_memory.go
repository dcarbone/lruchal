package lruchal

import (
	"container/list"
	"sync"
	"time"
)

type memoryCacheItem struct {
	mu *sync.Mutex

	key   interface{}
	value interface{}

	expired bool
	kill    chan struct{}
}

func newMemoryCachedItem(key, value interface{}, ttl time.Duration) *memoryCacheItem {
	ci := &memoryCacheItem{
		mu:    new(sync.Mutex),
		key:   key,
		value: value,
		kill:  make(chan struct{}),
	}

	go ci.expire(ttl)

	return ci
}

func (ci *memoryCacheItem) Key() interface{} {
	return ci.key
}

func (ci *memoryCacheItem) Value() interface{} {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	return ci.value
}

func (ci *memoryCacheItem) term() {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	if !ci.expired {
		close(ci.kill)
		ci.expired = true
		ci.value = nil
		ci.key = nil
	}
}

func (ci *memoryCacheItem) expire(ttl time.Duration) {
	timer := time.NewTimer(ttl)
	defer timer.Stop()
	select {
	case <-timer.C:
		ci.mu.Lock()
		ci.expired = true
		ci.value = nil
		ci.key = nil
		ci.mu.Unlock()
	case <-ci.kill:
	}
}

type MemoryCache struct {
	mu       *sync.Mutex
	list     *list.List
	elements map[interface{}]*list.Element
	maxSize  int
}

func NewMemoryCache(maxSize int) *MemoryCache {
	if maxSize <= 0 {
		panic("maxSize must be greater than 0")
	}
	cc := &MemoryCache{
		mu:       new(sync.Mutex),
		list:     list.New(),
		elements: make(map[interface{}]*list.Element, maxSize),
		maxSize:  maxSize,
	}

	cc.list.Init()

	return cc
}

func (cc *MemoryCache) Has(key interface{}) bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	_, ok := cc.elements[key]
	return ok
}

// Remove will attempt to remove a key from this cache, returning it's value.  Returns nil if key not found.
func (cc *MemoryCache) Remove(key interface{}) interface{} {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if elem, ok := cc.elements[key]; ok {
		item := cc.list.Remove(elem).(*memoryCacheItem)
		val := item.value
		delete(cc.elements, item.key)
		item.term()
		return val
	}
	return nil
}

// Put will perform an upsert on a key, potentially expunging the least recently used if there is no more room.
func (cc *MemoryCache) Put(key, value interface{}, ttl time.Duration) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if elem, ok := cc.elements[key]; ok {
		elem.Value.(*memoryCacheItem).term()
		elem.Value = newMemoryCachedItem(key, value, ttl)
		cc.list.MoveToFront(elem)
	} else {
		if cc.list.Len() == cc.maxSize {
			cc.list.Remove(cc.list.Back()).(*memoryCacheItem).term()
		}
		cc.elements[key] = cc.list.PushFront(newMemoryCachedItem(key, value, ttl))
	}
}

// Get will attempt to return a key value for you.  Will return nil if key is expired.
func (cc *MemoryCache) Get(key interface{}) interface{} {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if elem, ok := cc.elements[key]; ok {
		if !elem.Value.(*memoryCacheItem).expired {
			cc.list.MoveToFront(elem)
			return elem.Value.(*memoryCacheItem).value
		}
	}
	return nil
}

func (cc *MemoryCache) Len() int {
	return cc.list.Len()
}

// Expunge will remove expired keys from the cache
func (cc *MemoryCache) Expunge() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	expired := make([]interface{}, cc.Len())
	i := 0
	for key, elem := range cc.elements {
		if elem.Value.(*memoryCacheItem).expired {
			expired[i] = key
			cc.list.Remove(elem)
			i++
		}
	}

	for _, key := range expired {
		if key == nil {
			return
		}
		delete(cc.elements, key)
	}
}
