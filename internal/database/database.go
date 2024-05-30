package database

import (
	_ "embed"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Manager struct {
	ConnectionString   string
	Db                 *gorm.DB
	CreateIfNotExists  bool
	ActivateGormLogger bool
}

func NewDatabase(connectionString string, createIfNotExists bool, activateGormLogger bool) Manager {
	return Manager{
		ConnectionString:   connectionString,
		Db:                 nil,
		CreateIfNotExists:  createIfNotExists,
		ActivateGormLogger: activateGormLogger,
	}
}

func (dbMgr *Manager) Open() error {
	var db *gorm.DB
	var err error
	if dbMgr.ActivateGormLogger {
		db, err = gorm.Open(sqlite.Open(dbMgr.ConnectionString), &gorm.Config{})
		if err != nil {
			return err
		}
	} else {
		db, err = gorm.Open(sqlite.Open(dbMgr.ConnectionString), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			return err
		}
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
