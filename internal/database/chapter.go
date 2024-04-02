package database

import (
	"database/sql"
	"sync"
)

type Chapter struct {
	Id            int
	Manga         *Manga
	Url           string
	Name          string
	Number        int
	TimeStampUnix int64
}

type ChapterTable[K comparable] struct {
	mutex    sync.Mutex
	chapters map[K]Chapter
	updated  map[K]DbStatus
}

func (c *ChapterTable[K]) Get(key K) (Chapter, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	val, ok := c.chapters[key]
	return val, ok
}

func (c *ChapterTable[K]) Set(key K, new Chapter) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	val, ok := c.updated[key]
	if ok && val == Loaded {
		c.updated[key] = Updated
	} else {
		c.updated[key] = New
	}
	c.chapters[key] = new
}

func (c *ChapterTable[K]) All() []Chapter {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	res := make([]Chapter, len(c.chapters))
	counter := 0
	for _, chapter := range c.chapters {
		res[counter] = chapter
		counter++
	}
	return res
}

func (c *ChapterTable[K]) Save(db *sql.DB) error {
	//TODO implement me
	panic("implement me")
}

func (c *ChapterTable[K]) Load(db *sql.DB) error {
	//TODO implement me
	panic("implement me")
}
