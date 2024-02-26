package database

import (
	"bytes"
	"mangaGetter/internal/provider"
)

type Manga struct {
	Id            int
	Provider      provider.Provider
	Rating        int
	Title         string
	TimeStampUnix int64
	Thumbnail     *bytes.Buffer

	// Not in DB
	LatestChapter *Chapter
}
