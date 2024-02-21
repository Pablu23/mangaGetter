package main

import (
	"database/sql"
	_ "embed"
	_ "github.com/mattn/go-sqlite3"
	"sync"
)

type Manga struct {
	Id            int
	Title         string
	TimeStampUnix int64

	// Not in DB
	LatestChapter Option[Chapter]
}

type Chapter struct {
	Id            int
	Manga         Option[Manga]
	Url           string
	Name          string
	Number        int
	TimeStampUnix int64
}

type DatabaseManager struct {
	ConnectionString string
	db               *sql.DB

	// TODO(Pablu23): This probably needs a mutex?
	Mangas   *ConcurrentMap[int, Manga]
	Chapters *ConcurrentMap[int, Chapter]

	CreateIfNotExists bool
}

func NewDatabase(connectionString string, createIfNotExists bool) DatabaseManager {
	return DatabaseManager{
		ConnectionString: connectionString,

		Mangas: &ConcurrentMap[int, Manga]{
			lock:  sync.RWMutex{},
			dirty: make(map[int]Option[Manga]),
		},
		Chapters: &ConcurrentMap[int, Chapter]{
			lock:  sync.RWMutex{},
			dirty: make(map[int]Option[Chapter]),
		},
		CreateIfNotExists: createIfNotExists,
	}
}

func (dbMgr *DatabaseManager) Open() error {
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

func (dbMgr *DatabaseManager) Close() error {
	err := dbMgr.db.Close()
	if err != nil {
		return err
	}

	dbMgr.Mangas = nil
	dbMgr.Chapters = nil
	dbMgr.db = nil
	return nil
}

func (dbMgr *DatabaseManager) Save() error {
	db := dbMgr.db

	for _, m := range dbMgr.Mangas.All() {
		count := 0
		err := db.QueryRow("SELECT COUNT(*) FROM Manga where ID = ?", m.Id).Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
			_, err := db.Exec("INSERT INTO Manga(ID, Title, TimeStampUnixEpoch) values(?, ?, ?)", m.Id, m.Title, m.TimeStampUnix)
			if err != nil {
				return err
			}
		} else {
			_, err := db.Exec("UPDATE Manga set Title = ?, TimeStampUnixEpoch = ? WHERE ID = ?", m.Title, m.TimeStampUnix, m.Id)
			if err != nil {
				return err
			}
		}
	}

	for _, c := range dbMgr.Chapters.All() {
		count := 0
		err := db.QueryRow("SELECT COUNT(*) FROM Chapter where ID = ?", c.Id).Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
			_, err := db.Exec("INSERT INTO Chapter(ID, MangaID, Url, Name, Number, TimeStampUnixEpoch) VALUES (?, ?, ?, ?, ?, ?)", c.Id, c.Manga.Value.Id, c.Url, c.Name, c.Number, c.TimeStampUnix)
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

func (dbMgr *DatabaseManager) createDatabaseIfNotExists() error {
	_, err := dbMgr.db.Exec(createSql)
	return err
}

func (dbMgr *DatabaseManager) load() error {
	db := dbMgr.db
	rows, err := db.Query("SELECT * FROM Manga")
	if err != nil {
		return err
	}

	for rows.Next() {
		manga := Manga{}
		if err = rows.Scan(&manga.Id, &manga.Title, &manga.TimeStampUnix); err != nil {
			return err
		}
		dbMgr.Mangas.Set(manga.Id, manga)
	}

	rows, err = db.Query("SELECT * FROM Chapter")
	if err != nil {
		return err
	}

	for rows.Next() {
		chapter := Chapter{}
		var mangaID int
		if err = rows.Scan(&chapter.Id, &mangaID, &chapter.Url, &chapter.Name, &chapter.Number, &chapter.TimeStampUnix); err != nil {
			return err
		}

		// TODO: Does this actually work?
		manga := dbMgr.Mangas.Get(mangaID)
		chapter.Manga = Ok(manga)
		if !dbMgr.Mangas.Get(mangaID).LatestChapter.Set || dbMgr.Mangas.Get(mangaID).LatestChapter.Value.TimeStampUnix < chapter.TimeStampUnix {
			manga.LatestChapter = Ok(chapter)
			dbMgr.Mangas.Set(mangaID, manga)
		}

		dbMgr.Chapters.Set(chapter.Id, chapter)
	}

	return nil
}
