package view

import "github.com/pablu23/mangaGetter/internal/database"

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
	Number       string
	LastNumber   string
	LastTime     string
	Url          string
	ThumbnailUrl string
	Enabled      bool
}

type MenuViewModel struct {
	Archive  bool
	Settings map[string]database.Setting
	Mangas   []MangaViewModel
}
