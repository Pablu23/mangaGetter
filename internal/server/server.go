package server

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/google/uuid"
	"io"
	"mangaGetter/internal/database"
	"mangaGetter/internal/view"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Server struct {
	ContextManga  *database.Manga
	PrevViewModel *view.ImageViewModel
	CurrViewModel *view.ImageViewModel
	NextViewModel *view.ImageViewModel

	ImageBuffers map[string]*bytes.Buffer
	Mutex        *sync.Mutex

	NextSubUrl string
	CurrSubUrl string
	PrevSubUrl string

	IsFirst bool
	IsLast  bool

	DbMgr *database.Manager
}

func New(db *database.Manager) *Server {
	s := Server{
		ImageBuffers: make(map[string]*bytes.Buffer),
		DbMgr:        db,
		Mutex:        &sync.Mutex{},
	}

	return &s
}

func (s *Server) LoadNext() {
	c, err := s.ContextManga.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	next, err := s.ContextManga.Provider.GetNext(c)
	if err != nil {
		fmt.Println(err)
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	html, err := s.ContextManga.Provider.GetHtml(next)
	if err != nil {
		fmt.Println(err)
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		fmt.Println(err)
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	title, chapter, err := s.ContextManga.Provider.GetTitleAndChapter(next)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.NextViewModel = &view.ImageViewModel{Images: imagesNext, Title: full}
	s.NextSubUrl = next
	fmt.Println("Loaded next")
}

func (s *Server) LoadPrev() {
	c, err := s.ContextManga.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}
	prev, err := s.ContextManga.Provider.GetPrev(c)
	if err != nil {
		fmt.Println(err)
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}
	html, err := s.ContextManga.Provider.GetHtml(prev)
	if err != nil {
		fmt.Println(err)
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		fmt.Println(err)
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}

	title, chapter, err := s.ContextManga.Provider.GetTitleAndChapter(prev)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.PrevViewModel = &view.ImageViewModel{Images: imagesNext, Title: full}

	s.PrevSubUrl = prev
	fmt.Println("Loaded prev")
}

func (s *Server) LoadCurr() {
	html, err := s.ContextManga.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		panic(err)
	}

	imagesCurr, err := s.AppendImagesToBuf(html)

	title, chapter, err := s.ContextManga.Provider.GetTitleAndChapter(s.CurrSubUrl)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.CurrViewModel = &view.ImageViewModel{Images: imagesCurr, Title: full}
	fmt.Println("Loaded current")
}

func (s *Server) LoadThumbnail(mangaId int) (path string, err error) {
	strId := strconv.Itoa(mangaId)

	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if s.ImageBuffers[strId] != nil {
		return strId, nil
	}

	url, err := s.ContextManga.Provider.GetThumbnail(strconv.Itoa(mangaId))
	if err != nil {
		return "", err
	}
	ram, err := addFileToRam(url)
	if err != nil {
		return "", err
	}
	s.ImageBuffers[strId] = ram
	return strId, nil
}

func (s *Server) AppendImagesToBuf(html string) ([]view.Image, error) {
	imgList, err := s.ContextManga.Provider.GetImageList(html)
	if err != nil {
		return nil, err
	}

	images := make([]view.Image, len(imgList))

	wg := sync.WaitGroup{}
	for i, url := range imgList {
		wg.Add(1)
		go func(i int, url string, wg *sync.WaitGroup) {
			buf, err := addFileToRam(url)
			if err != nil {
				panic(err)
			}
			g := uuid.New()
			s.Mutex.Lock()
			s.ImageBuffers[g.String()] = buf
			s.Mutex.Unlock()
			images[i] = view.Image{Path: g.String(), Index: i}
			wg.Done()
		}(i, url, &wg)
	}

	wg.Wait()
	return images, nil
}

func addFileToRam(url string) (*bytes.Buffer, error) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(resp.Body)

	buf := new(bytes.Buffer)

	// Write the body to file
	_, err = io.Copy(buf, resp.Body)
	return buf, err
}
