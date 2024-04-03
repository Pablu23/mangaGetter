package database

import (
	"database/sql"
)

type Chapter struct {
	Id            int
	MangaId       int
	Url           string
	Name          string
	Number        string
	TimeStampUnix int64
}

func NewChapter(id int, mangaId int, url string, name string, number string, timeStampUnix int64) Chapter {
	return Chapter{
		Id:            id,
		MangaId:       mangaId,
		Url:           url,
		Name:          name,
		Number:        number,
		TimeStampUnix: timeStampUnix,
	}
}

func updateChapter(db *sql.DB, c *Chapter) error {
	_, err := db.Exec("UPDATE Chapter set Name = ?, Url = ?, Number = ?, TimeStampUnixEpoch = ? where ID = ?", c.Name, c.Url, c.Number, c.TimeStampUnix, c.Id)
	return err
}

func insertChapter(db *sql.DB, c *Chapter) error {
	_, err := db.Exec("INSERT INTO Chapter(ID, MangaID, Url, Name, Number, TimeStampUnixEpoch) VALUES (?, ?, ?, ?, ?, ?)", c.Id, c.MangaId, c.Url, c.Name, c.Number, c.TimeStampUnix)
	return err
}

func loadChapters(db *sql.DB) (map[int]Chapter, error) {
	rows, err := db.Query("SELECT Id, MangaID, Url, Name, Number, TimeStampUnixEpoch FROM Chapter")
	if err != nil {
		return nil, err
	}
	res := make(map[int]Chapter)

	for rows.Next() {
		chapter := Chapter{}
		if err = rows.Scan(&chapter.Id, &chapter.MangaId, &chapter.Url, &chapter.Name, &chapter.Number, &chapter.TimeStampUnix); err != nil {
			return nil, err
		}
		res[chapter.Id] = chapter
	}
	return res, err
}

func deleteChapter(db *sql.DB, key int) error {
	_, err := db.Exec("DELETE from Chapter where ID = ?", key)
	return err
}
