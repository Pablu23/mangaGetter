package view

import "mangaGetter/internal/database"

type Image struct {
	Path  string
	Index int
}

type ImageViewModel struct {
	Title  string
	Images []Image
}

type MangaViewModel struct {
	ID           int
	Title        string
	Number       int
	LastNumber   int
	LastTime     string
	Url          string
	ThumbnailUrl string
}

type MenuViewModel struct {
	Settings map[string]database.Setting
	Mangas   []MangaViewModel
}
