package database

import (
	_ "embed"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Manager struct {
	ConnectionString  string
	Db                *gorm.DB
	CreateIfNotExists bool
}

func NewDatabase(connectionString string, createIfNotExists bool) Manager {
	return Manager{
		ConnectionString:  connectionString,
		Db:                nil,
		CreateIfNotExists: createIfNotExists,
	}
}

func (dbMgr *Manager) Open() error {
	db, err := gorm.Open(sqlite.Open(dbMgr.ConnectionString), &gorm.Config{})
	if err != nil {
		return err
	}
	dbMgr.Db = db
	if dbMgr.CreateIfNotExists {
		err = dbMgr.createDatabaseIfNotExists()
		if err != nil {
			return err
		}
	}
	return err
}

func (dbMgr *Manager) Close() error {
	sql, err := dbMgr.Db.DB()
	if err != nil {
		return err
	}
	err = sql.Close()
	if err != nil {
		return err
	}
	dbMgr.Db = nil
	return nil
}

func (dbMgr *Manager) Delete(mangaId int) {
	dbMgr.Db.Delete(&Manga{}, mangaId)
}

func (dbMgr *Manager) createDatabaseIfNotExists() error {
	err := dbMgr.Db.AutoMigrate(&Manga{}, &Chapter{}, &Setting{})
	return err
}
