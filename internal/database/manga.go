package database

import (
	"bytes"
	"database/sql"
	"sync"
)

type Manga struct {
	Id             int
	Title          string
	TimeStampUnix  int64
	Thumbnail      *bytes.Buffer
	LastChapterNum int

	// Not in DB
	LatestChapter *Chapter
}

type MangaTable[K comparable] struct {
	mutex   sync.Mutex
	mangas  map[K]Manga
	updated map[K]DbStatus
}

func (m *MangaTable[K]) Get(key K) (Manga, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	val, ok := m.mangas[key]
	return val, ok
}

func (m *MangaTable[K]) Set(key K, new Manga) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	val, ok := m.updated[key]
	if ok && val == Loaded {
		m.updated[key] = Updated
	} else {
		m.updated[key] = New
	}
	m.mangas[key] = new
}

func (m *MangaTable[K]) All() []Manga {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	res := make([]Manga, len(m.mangas))
	counter := 0
	for _, manga := range m.mangas {
		res[counter] = manga
		counter++
	}
	return res
}

func (m *MangaTable[K]) Save(db *sql.DB) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for k, status := range m.updated {
		if status == Loaded {
			continue
		} else if status == Updated {
			manga := m.mangas[k]
			err := updateManga(db, &manga)
			if err != nil {
				return err
			}
		} else {
			manga := m.mangas[k]
			err := insertManga(db, &manga)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MangaTable[K]) Load(db *sql.DB) error {
	panic("")
}

func updateManga(db *sql.DB, m *Manga) error {
	const cmd = "UPDATE Manga set Title = ?, TimeStampUnixEpoch = ?, Thumbnail = ?, LatestAvailableChapter = ? WHERE ID = ?"
	_, err := db.Exec(cmd, m.Title, m.TimeStampUnix, m.Thumbnail.Bytes(), m.LastChapterNum, m.Id)
	return err
}

func insertManga(db *sql.DB, manga *Manga) error {
	const cmd = "INSERT INTO Manga(ID, Title, TimeStampUnixEpoch, Thumbnail, LatestAvailableChapter) values(?, ?, ?, ?, ?)"
	_, err := db.Exec(cmd, manga.Id, manga.Title, manga.TimeStampUnix, manga.Thumbnail.Bytes(), manga.LastChapterNum)
	return err
}
