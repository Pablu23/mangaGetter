package database

import (
	"bytes"
	"database/sql"
)

type Manga struct {
	Id             int
	Title          string
	TimeStampUnix  int64
	Thumbnail      *bytes.Buffer
	LastChapterNum int
}

func NewManga(id int, title string, timeStampUnix int64) Manga {
	return Manga{
		Id:            id,
		Title:         title,
		TimeStampUnix: timeStampUnix,
	}
}

// GetLatestChapter TODO: Cache this somehow
func (m *Manga) GetLatestChapter(chapters *DbTable[int, Chapter]) (*Chapter, bool) {
	c := chapters.All()

	highest := int64(0)
	index := 0
	for i, chapter := range c {
		if chapter.MangaId == m.Id && highest < chapter.TimeStampUnix {
			highest = chapter.TimeStampUnix
			index = i
		}
	}

	if highest == 0 {
		return nil, false
	}

	return &c[index], true
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

func loadMangas(db *sql.DB) (map[int]Manga, error) {
	rows, err := db.Query("SELECT Id, Title, TimeStampUnixEpoch, Thumbnail, LatestAvailableChapter FROM Manga")
	if err != nil {

		return nil, err
	}
	res := make(map[int]Manga)

	for rows.Next() {
		manga := Manga{}
		var thumbnail []byte
		if err = rows.Scan(&manga.Id, &manga.Title, &manga.TimeStampUnix, &thumbnail, &manga.LastChapterNum); err != nil {
			return nil, err
		}
		if len(thumbnail) != 0 {
			manga.Thumbnail = bytes.NewBuffer(thumbnail)
		}

		res[manga.Id] = manga
	}

	return res, nil
}

func deleteManga(db *sql.DB, key int) error {
	_, err := db.Exec("DELETE from Chapter where ID = ?", key)
	return err
}
