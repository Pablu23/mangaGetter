package utils

import "sync"

type ConcurrentMap[K comparable, Value any] struct {
	dirty map[K]Value
	count int
	mutex *sync.RWMutex
}

func NewConcurrentMap[K comparable, V any]() ConcurrentMap[K, V] {
	return ConcurrentMap[K, V]{
		dirty: make(map[K]V),
		count: 0,
		mutex: &sync.RWMutex{},
	}
}

func (c *ConcurrentMap[K, Value]) Get(key K) (Value, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	val, ok := c.dirty[key]

	return val, ok
}

func (c *ConcurrentMap[K, Value]) Set(key K, val Value) {
  c.mutex.Lock()
  defer c.mutex.Unlock()
  c.dirty[key] = val
}
