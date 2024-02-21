package main

import "sync"

type ConcurrentMap[T comparable, K any] struct {
	lock  sync.RWMutex
	dirty map[T]Option[K]
}

func (cm *ConcurrentMap[T, K]) Get(index T) K {
	var result K

	cm.lock.Lock()
	result = *cm.dirty[index].Value
	cm.lock.Unlock()

	return result
}

func (cm *ConcurrentMap[T, K]) Set(index T, value K) {
	cm.lock.Lock()
	cm.dirty[index] = Ok(value)
	cm.lock.Unlock()
}

func (cm *ConcurrentMap[T, K]) SetIfNil(index T, value K) bool {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	c := cm.dirty[index]
	if !c.Set {
		cm.dirty[index] = Ok(value)
		return true
	}
	return false
}

func (cm *ConcurrentMap[T, K]) All() []K {
	cm.lock.Lock()
	l := len(cm.dirty)
	result := make([]K, l)
	counter := 0
	for _, d := range cm.dirty {
		result[counter] = *d.Value
		counter++
	}
	cm.lock.Lock()
	return result
}
