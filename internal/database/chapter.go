package database

type Chapter struct {
	Id                 int
	Manga              *Manga
	InternalIdentifier string
	Url                string
	Name               string
	Number             int
	TimeStampUnix      int64
}
