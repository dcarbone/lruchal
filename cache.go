package lruchal

import (
	"container/list"
	"sync"
	"time"
)

type Item struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	TTL   string      `json:"ttl"`
}

type cachedItem struct {
	mu *sync.Mutex

	key   interface{}
	value interface{}

	expired bool
	kill    chan struct{}
}

func newCachedItem(key, value interface{}, ttl time.Duration) *cachedItem {
	ci := &cachedItem{
		mu:    new(sync.Mutex),
		key:   key,
		value: value,
		kill:  make(chan struct{}),
	}

	go ci.expire(ttl)

	return ci
}

func (ci *cachedItem) Key() interface{} {
	return ci.key
}

func (ci *cachedItem) Value() interface{} {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	return ci.value
}

func (ci *cachedItem) term() {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	if !ci.expired {
		close(ci.kill)
		ci.expired = true
		ci.value = nil
		ci.key = nil
	}
}

func (ci *cachedItem) expire(ttl time.Duration) {
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

type Cache struct {
	mu       *sync.Mutex
	list     *list.List
	elements map[interface{}]*list.Element
	maxSize  int
}

func NewCache(maxSize int) *Cache {
	if maxSize <= 0 {
		panic("maxSize must be greater than 0")
	}
	cc := &Cache{
		mu:       new(sync.Mutex),
		list:     list.New(),
		elements: make(map[interface{}]*list.Element, maxSize),
		maxSize:  maxSize,
	}

	cc.list.Init()

	return cc
}

func (cc *Cache) Has(key interface{}) bool {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	_, ok := cc.elements[key]
	return ok
}

// Remove will attempt to remove a key from this cache, returning it's value.  Returns nil if key not found.
func (cc *Cache) Remove(key interface{}) interface{} {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if elem, ok := cc.elements[key]; ok {
		item := cc.list.Remove(elem).(*cachedItem)
		val := item.value
		delete(cc.elements, item.key)
		item.term()
		return val
	}
	return nil
}

// Put will perform an upsert on a key, potentially expunging the least recently used if there is no more room.
func (cc *Cache) Put(key, value interface{}, ttl time.Duration) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if elem, ok := cc.elements[key]; ok {
		elem.Value.(*cachedItem).term()
		elem.Value = newCachedItem(key, value, ttl)
		cc.list.MoveToFront(elem)
	} else {
		if cc.list.Len() == cc.maxSize {
			cc.list.Remove(cc.list.Back()).(*cachedItem).term()
		}
		cc.elements[key] = cc.list.PushFront(newCachedItem(key, value, ttl))
	}
}

// Get will attempt to return a key value for you.  Will return nil if key is expired.
func (cc *Cache) Get(key interface{}) interface{} {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if elem, ok := cc.elements[key]; ok {
		if !elem.Value.(*cachedItem).expired {
			cc.list.MoveToFront(elem)
			return elem.Value.(*cachedItem).value
		}
	}
	return nil
}

func (cc *Cache) Len() int {
	return cc.list.Len()
}

// Expunge will remove expired keys from the cache
func (cc *Cache) Expunge() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	expired := make([]interface{}, cc.Len())
	i := 0
	for key, elem := range cc.elements {
		if elem.Value.(*cachedItem).expired {
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
