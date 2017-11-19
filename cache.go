package lruchal

import "time"

type Cache interface {
	Has(key interface{}) bool
	Remove(key interface{}) interface{}
	Put(key, value interface{}, ttl time.Duration)
	Get(key interface{}) interface{}
	Len() int
	Expunge()
}
