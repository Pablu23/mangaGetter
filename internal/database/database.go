package database

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"sync"

	_ "github.com/mattn/go-sqlite3"
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

type Chapter struct {
	Id            int
	Manga         *Manga
	Url           string
	Name          string
	Number        int
	TimeStampUnix int64
}

type Manager struct {
	ConnectionString string
	db               *sql.DB

	Rw       *sync.Mutex
	Mangas   map[int]*Manga
	Chapters map[int]*Chapter

	CreateIfNotExists bool
}

func NewDatabase(connectionString string, createIfNotExists bool) Manager {
	return Manager{
		ConnectionString:  connectionString,
		Rw:                &sync.Mutex{},
		Mangas:            make(map[int]*Manga),
		Chapters:          make(map[int]*Chapter),
		CreateIfNotExists: createIfNotExists,
	}
}

func (dbMgr *Manager) Open() error {
	db, err := sql.Open("sqlite3", dbMgr.ConnectionString)
	if err != nil {
		return err
	}
	dbMgr.db = db
	if dbMgr.CreateIfNotExists {
		err = dbMgr.createDatabaseIfNotExists()
		if err != nil {
			return err
		}
	}
	err = dbMgr.load()
	return err
}

func (dbMgr *Manager) Close() error {
	err := dbMgr.db.Close()
	if err != nil {
		return err
	}

	dbMgr.Mangas = nil
	dbMgr.Chapters = nil
	dbMgr.db = nil
	return nil
}

func (dbMgr *Manager) Delete(mangaId int) error {
	db := dbMgr.db
	fmt.Println("Locking Rw in database.go:84")
	dbMgr.Rw.Lock()
	defer func() {
		fmt.Println("Unlocking Rw in database.go:87")
		dbMgr.Rw.Unlock()
	}()

	_, err := db.Exec("DELETE from Chapter where MangaID = ?", mangaId)
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE  from Manga where ID = ?", mangaId)
	if err != nil {
		return err
	}

	for i, chapter := range dbMgr.Chapters {
		if chapter.Manga.Id == mangaId {
			delete(dbMgr.Chapters, i)
		}
	}
	delete(dbMgr.Mangas, mangaId)
	return nil
}

func (dbMgr *Manager) Save() error {
	db := dbMgr.db

	fmt.Println("Locking Rw in database.go:113")
	dbMgr.Rw.Lock()
	defer func() {
		fmt.Println("Unlocking Rw in database.go:116")
		dbMgr.Rw.Unlock()
	}()
	for _, m := range dbMgr.Mangas {
		count := 0
		err := db.QueryRow("SELECT COUNT(*) FROM Manga where ID = ?", m.Id).Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
			if m.Thumbnail != nil {
				_, err := db.Exec("INSERT INTO Manga(ID, Title, TimeStampUnixEpoch, Thumbnail, LatestAvailableChapter) values(?, ?, ?, ?, ?)", m.Id, m.Title, m.TimeStampUnix, m.Thumbnail.Bytes(), m.LastChapterNum)
				if err != nil {
					return err
				}
			} else {
				_, err := db.Exec("INSERT INTO Manga(ID, Title, TimeStampUnixEpoch, LatestAvailableChapter) values(?, ?, ?, ?)", m.Id, m.Title, m.TimeStampUnix, m.LastChapterNum)
				if err != nil {
					return err
				}
			}
		} else {
			tSet := 0
			err := db.QueryRow("SELECT COUNT(*) from Manga where ID = ? and Thumbnail IS NOT NULL", m.Id).Scan(&tSet)
			if err != nil {
				return err
			}

			if tSet != 0 {
				_, err = db.Exec("UPDATE Manga set Title = ?, TimeStampUnixEpoch = ?, LatestAvailableChapter = ? WHERE ID = ?", m.Title, m.TimeStampUnix, m.LastChapterNum, m.Id)
				if err != nil {
					return err
				}
			} else {
				_, err = db.Exec("UPDATE Manga set Title = ?, TimeStampUnixEpoch = ?, Thumbnail = ?, LatestAvailableChapter = ? WHERE ID = ?", m.Title, m.TimeStampUnix, m.Thumbnail.Bytes(), m.LastChapterNum, m.Id)
				if err != nil {
					return err
				}
			}
		}
	}

	for _, c := range dbMgr.Chapters {
		count := 0
		err := db.QueryRow("SELECT COUNT(*) FROM Chapter where ID = ?", c.Id).Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
			_, err := db.Exec("INSERT INTO Chapter(ID, MangaID, Url, Name, Number, TimeStampUnixEpoch) VALUES (?, ?, ?, ?, ?, ?)", c.Id, c.Manga.Id, c.Url, c.Name, c.Number, c.TimeStampUnix)
			if err != nil {
				return err
			}
		} else {
			_, err = db.Exec("UPDATE Chapter set Name = ?, Url = ?, Number = ?, TimeStampUnixEpoch = ? where ID = ?", c.Name, c.Url, c.Number, c.TimeStampUnix, c.Id)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

//go:embed createDb.sql
var createSql string

func (dbMgr *Manager) createDatabaseIfNotExists() error {
	_, err := dbMgr.db.Exec(createSql)
	return err
}

func (dbMgr *Manager) load() error {
	db := dbMgr.db

	fmt.Println("Locking Rw in database.go:180")
	dbMgr.Rw.Lock()
	defer func() {
		fmt.Println("Unlocking Rw in database.go:183")
		dbMgr.Rw.Unlock()
	}()

	rows, err := db.Query("SELECT Id, Title, TimeStampUnixEpoch, Thumbnail, LatestAvailableChapter FROM Manga")
	if err != nil {
		return err
	}

	for rows.Next() {
		manga := Manga{}
		var thumbnail []byte
		if err = rows.Scan(&manga.Id, &manga.Title, &manga.TimeStampUnix, &thumbnail, &manga.LastChapterNum); err != nil {
			return err
		}
		if len(thumbnail) != 0 {
			manga.Thumbnail = bytes.NewBuffer(thumbnail)
		}

		latestChapter := db.QueryRow("SELECT Id, Url, Name, Number, TimeStampUnixEpoch FROM Chapter where MangaID = ? ORDER BY TimeStampUnixEpoch desc LIMIT 1", manga.Id)
		chapter := Chapter{}
		if err = latestChapter.Scan(&chapter.Id, &chapter.Url, &chapter.Name, &chapter.Number, &chapter.TimeStampUnix); err != nil {
			return err
		}
		chapter.Manga = &manga
		manga.LatestChapter = &chapter
		dbMgr.Chapters[chapter.Id] = &chapter
		dbMgr.Mangas[manga.Id] = &manga
	}
	return nil
}
