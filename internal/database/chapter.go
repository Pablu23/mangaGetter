package database

type Chapter struct {
	Id            int `gorm:"primary_key;autoIncrement;"`
	ChapterId     int
	Url           string
	Name          string
	Number        string
	TimeStampUnix int64
	MangaId       int
	UserId        int
}

func NewChapter(id int, mangaId int, userId int, url string, name string, number string, timeStampUnix int64) Chapter {
	return Chapter{
		ChapterId:     id,
		Url:           url,
		Name:          name,
		Number:        number,
		TimeStampUnix: timeStampUnix,
		MangaId:       mangaId,
    UserId: userId,
	}
}
