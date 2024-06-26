package database

type Chapter struct {
	Id            int `gorm:"primary_key;AUTO_INCREMENT"`
	Url           string
	Name          string
	Number        string
	TimeStampUnix int64
	MangaId       int
}

func NewChapter(id int, mangaId int, url string, name string, number string, timeStampUnix int64) Chapter {
	return Chapter{
		Id:            id,
		Url:           url,
		Name:          name,
		Number:        number,
		TimeStampUnix: timeStampUnix,
		MangaId:       mangaId,
	}
}
