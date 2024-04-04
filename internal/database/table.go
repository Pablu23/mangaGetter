package database

import (
	"database/sql"
	"sync"
)

type DbStatus uint8

const (
	New DbStatus = iota
	Loaded
	Updated
)

type DbTable[K comparable, T any] struct {
	mutex      sync.Mutex
	items      map[K]T
	updated    map[K]DbStatus
	updateFunc func(db *sql.DB, value *T) error
	insertFunc func(db *sql.DB, value *T) error
	loadFunc   func(db *sql.DB) (map[K]T, error)
	deleteFunc func(db *sql.DB, key K) error
}

func NewDbTable[K comparable, T any](updateFunc func(db *sql.DB, value *T) error,
	insertFunc func(db *sql.DB, value *T) error,
	loadFunc func(db *sql.DB) (map[K]T, error),
	deleteFunc func(db *sql.DB, key K) error,
) DbTable[K, T] {
	return DbTable[K, T]{
		mutex:      sync.Mutex{},
		items:      make(map[K]T),
		updated:    make(map[K]DbStatus),
		updateFunc: updateFunc,
		insertFunc: insertFunc,
		loadFunc:   loadFunc,
		deleteFunc: deleteFunc,
	}
}

func (d *DbTable[K, T]) Get(key K) (T, bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	val, ok := d.items[key]
	return val, ok
}

func (d *DbTable[K, T]) Set(key K, new T) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	val, ok := d.updated[key]
	if ok && val == Loaded || val == Updated {
		d.updated[key] = Updated
	} else {
		d.updated[key] = New
	}
	d.items[key] = new
}

func (d *DbTable[K, T]) All() []T {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	res := make([]T, len(d.items))
	counter := 0
	for _, manga := range d.items {
		res[counter] = manga
		counter++
	}
	return res
}

func (d *DbTable[K, T]) Map() map[K]T {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	res := make(map[K]T, len(d.items))
	for k, manga := range d.items {
		res[k] = manga
	}
	return res
}

func (d *DbTable[K, T]) First(filter func(match T) bool) (key K, value T, ok bool) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	for k, manga := range d.items {
		if filter(manga) {
			return k, manga, true
		}
	}

	return *new(K), *new(T), false
}

func (d *DbTable[K, T]) Where(filter func(match T) bool) map[K]T {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	res := make(map[K]T, len(d.items))
	for k, manga := range d.items {
		if filter(manga) {
			res[k] = manga
		}
	}
	return res
}

func (d *DbTable[K, T]) Delete(db *sql.DB, key K) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	err := d.deleteFunc(db, key)
	if err == nil {
		delete(d.items, key)
	}
	return err
}

func (d *DbTable[K, T]) Save(db *sql.DB) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	for k, status := range d.updated {
		switch status {
		case Loaded:
			continue
		case Updated:
			item := d.items[k]
			err := d.updateFunc(db, &item)
			if err != nil {
				return err
			}
			d.updated[k] = Loaded
		case New:
			item := d.items[k]
			err := d.insertFunc(db, &item)
			if err != nil {
				return err
			}
			d.updated[k] = Loaded
		}
	}
	return nil
}

func (d *DbTable[K, T]) Load(db *sql.DB) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	res, err := d.loadFunc(db)
	if err != nil {
		return err
	}
	d.items = res
	for k := range d.items {
		d.updated[k] = Loaded
	}
	return nil
}
