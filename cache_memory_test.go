package lruchal_test

import (
	"github.com/dcarbone/lruchal"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestMemoryCache(t *testing.T) {
	var v interface{}

	t.Run("InvalidMaxSizePanics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Log("Expected a panic")
				t.FailNow()
			}
		}()
		lruchal.NewMemoryCache(-1)
	})

	t.Run("Put", func(t *testing.T) {
		cache := lruchal.NewMemoryCache(100)
		cache.Put("key1", "value1", time.Millisecond)

		v = cache.Get("key1")
		if s, ok := v.(string); ok {
			if s != "value1" {
				t.Logf("Expected value to be \"value1\", saw \"%s\"", s)
				t.FailNow()
			}
		} else {
			t.Logf("Expected value to be string, saw \"%s\"", reflect.TypeOf(v))
			t.FailNow()
		}

		time.Sleep(1250 * time.Microsecond)
		v = cache.Get("key1")
		if v != nil {
			t.Logf("key1 should be dead (nil), saw %#v", v)
			t.FailNow()
		}
	})

	t.Run("Has", func(t *testing.T) {
		container := lruchal.NewMemoryCache(100)
		container.Put("key1", "value1", time.Second)
		if !container.Has("key1") {
			t.Logf("Expected container to have key \"%s\"", "key1")
			t.FailNow()
		}
	})

	t.Run("Remove", func(t *testing.T) {
		container := lruchal.NewMemoryCache(100)
		container.Put("key1", "value1", time.Second)
		if !container.Has("key1") {
			t.Logf("Expected container to have key \"%s\"", "key1")
			t.FailNow()
		}
		v := container.Remove("key1")
		if s, ok := v.(string); ok {
			if s != "value1" {
				t.Logf("Expected remove to return \"value1\", saw \"%s\"", s)
				t.FailNow()
			}
		} else {
			t.Logf("Expected remove to return string, got \"%s\"", reflect.TypeOf(v))
			t.FailNow()
		}
		if container.Has("key1") {
			t.Log("remove did not actually remove key")
			t.FailNow()
		}
	})

	t.Run("Expunge", func(t *testing.T) {
		container := lruchal.NewMemoryCache(100)
		container.Put("short1", "short1value", time.Microsecond)
		container.Put("long1", "long1value", time.Second)
		container.Put("short2", "short2value", time.Microsecond)
		container.Put("long2", "long2value", time.Second)
		container.Put("long3", "long3value", time.Second)

		if l := container.Len(); l != 5 {
			t.Logf("Expected container to have len 5, saw \"%d\"", l)
			t.FailNow()
		}

		time.Sleep(500 * time.Microsecond)

		container.Expunge()

		if l := container.Len(); l != 3 {
			t.Logf("Expected expunge to remove 2 elements for len 3, saw len \"%d\"", l)
			t.FailNow()
		}
	})
}

func BenchmarkMemoryCache(b *testing.B) {
	maxSize := 100
	container := lruchal.NewMemoryCache(maxSize)
	wg := new(sync.WaitGroup)
	s := maxSize * 2
	wg.Add(s)
	for i := 0; i < s; i++ {
		if i%2 == 0 {
			go func() {
				for i := 0; i < 1000; i++ {
					container.Put(rand.Intn(maxSize), i, 500*time.Microsecond)
				}
				wg.Done()
			}()
		} else {
			go func() {
				for i := 0; i < 1000; i++ {
					container.Get(rand.Intn(maxSize))
				}
				wg.Done()
			}()
		}
	}
	wg.Wait()
}
