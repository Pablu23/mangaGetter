package view

type Image struct {
	Path  string
	Index int
}

type ImageViewModel struct {
	Title  string
	Images []Image
}

type MangaViewModel struct {
	Title    string
	Number   int
	LastTime string
	Url      string
}

type MenuViewModel struct {
	Mangas []MangaViewModel
}
