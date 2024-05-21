package database

type MangaDefinition struct {
	Id             int `gorm:"primary_key;AUTO_INCREMENT"`
	Title          string
	TimeStampUnix  int64
	Thumbnail      []byte
	LastChapterNum string
	// Chapters       []Chapter
	//`gorm:"foreignkey:MangaID"`
}

type Manga struct {
	Id                int `gorm:"primary_key;AUTO_INCREMENT"`
	MangaDefinitionId int
  Definition        MangaDefinition `gorm:"foreignKey:MangaDefinitionId"`
	UserId            int
	TimeStampUnix     int64
	Chapters          []Chapter `gorm:"foreignKey:MangaId"`
}

func NewMangaDefinition(id int, title string, timeStampUnix int64) MangaDefinition {
	return MangaDefinition{
		Id:             id,
		Title:          title,
		TimeStampUnix:  timeStampUnix,
		LastChapterNum: "",
	}
}

// GetLatestChapter TODO: Cache this somehow
func (m *Manga) GetLatestChapter() (*Chapter, bool) {
	// highest := int64(0)
	// index := 0
	// for i, chapter := range m.Chapters {
	// 	if chapter.MangaId == m.Manga.Id && highest < chapter.TimeStampUnix {
	// 		highest = chapter.TimeStampUnix
	// 		index = i
	// 	}
	// }

	// if highest == 0 {
	return nil, false
	// }

	// return &m.Chapters[index], true

	//result := db.Where("manga.id = ?", m.Id).Order("TimeStampUnix desc").Take(&chapter)
	//if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
	//	return &chapter, true, result.Error
	//} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
	//	return &chapter, false, nil
	//} else {
	//	return &chapter, true, nil
	//}
}
