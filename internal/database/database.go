package database

import (
	"database/sql"
	_ "embed"
	_ "github.com/mattn/go-sqlite3"
)

type Manager struct {
	ConnectionString  string
	db                *sql.DB
	Mangas            DbTable[int, Manga]
	Chapters          DbTable[int, Chapter]
	Settings          DbTable[string, Setting]
	CreateIfNotExists bool
}

func NewDatabase(connectionString string, createIfNotExists bool) Manager {
	return Manager{
		ConnectionString:  connectionString,
		db:                nil,
		Mangas:            NewDbTable(updateManga, insertManga, loadMangas, deleteManga),
		Chapters:          NewDbTable(updateChapter, insertChapter, loadChapters, deleteChapter),
		Settings:          NewDbTable(updateSetting, insertSetting, loadSettings, deleteSetting),
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

	dbMgr.db = nil
	return nil
}

func (dbMgr *Manager) Delete(mangaId int) error {
	dbMgr.Mangas.Get(mangaId)
	err := dbMgr.Mangas.Delete(dbMgr.db, mangaId)
	if err != nil {
		return err
	}

	chapters := dbMgr.Chapters.All()
	for i, chapter := range chapters {
		if chapter.MangaId == mangaId {
			err := dbMgr.Chapters.Delete(dbMgr.db, i)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (dbMgr *Manager) Save() error {
	err := dbMgr.Mangas.Save(dbMgr.db)
	if err != nil {
		return err
	}
	err = dbMgr.Chapters.Save(dbMgr.db)
	if err != nil {
		return err
	}

	return dbMgr.Settings.Save(dbMgr.db)
}

//go:embed createDb.sql
var createSql string

func (dbMgr *Manager) createDatabaseIfNotExists() error {
	_, err := dbMgr.db.Exec(createSql)
	return err
}

func (dbMgr *Manager) load() error {
	err := dbMgr.Chapters.Load(dbMgr.db)
	if err != nil {
		return err
	}

	err = dbMgr.Mangas.Load(dbMgr.db)
	if err != nil {
		return err
	}

	err = dbMgr.Settings.Load(dbMgr.db)
	if err != nil {
		return err
	}
	initSettings(&dbMgr.Settings)

	return nil
}
