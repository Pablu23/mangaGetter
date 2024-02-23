package provider

type Provider interface {
	GetImageList(html string) (imageUrls []string, err error)
	GetHtml(url string) (html string, err error)
	GetNext(html string) (url string, err error)
	GetPrev(html string) (url string, err error)
	GetTitleAndChapter(url string) (title string, chapter string, err error)
	GetTitleIdAndChapterId(url string) (titleId int, chapterId int, err error)
	GetThumbnail(mangaId string) (thumbnailUrl string, err error)
}
