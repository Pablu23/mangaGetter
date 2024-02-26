package server

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"mangaGetter/internal/database"
	"mangaGetter/internal/provider"
	"mangaGetter/internal/view"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Server struct {
	PrevViewModel *view.ImageViewModel
	CurrViewModel *view.ImageViewModel
	NextViewModel *view.ImageViewModel

	ImageBuffers map[string]*bytes.Buffer
	Mutex        *sync.Mutex

	NextSubUrl string
	CurrSubUrl string
	PrevSubUrl string

	Provider provider.Provider

	IsFirst bool
	IsLast  bool

	DbMgr *database.Manager
}

func New(provider provider.Provider, db *database.Manager) *Server {
	s := Server{
		ImageBuffers: make(map[string]*bytes.Buffer),
		Provider:     provider,
		DbMgr:        db,
		Mutex:        &sync.Mutex{},
	}

	s.AddIco()

	return &s
}

func (s *Server) LoadNext() {
	c, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	next, err := s.Provider.GetNext(c)
	if err != nil {
		fmt.Println(err)
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	html, err := s.Provider.GetHtml(next)
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

	title, chapter, err := s.Provider.GetTitleAndChapter(next)
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
	c, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}
	prev, err := s.Provider.GetPrev(c)
	if err != nil {
		fmt.Println(err)
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}
	html, err := s.Provider.GetHtml(prev)
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

	title, chapter, err := s.Provider.GetTitleAndChapter(prev)
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
	html, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		panic(err)
	}

	imagesCurr, err := s.AppendImagesToBuf(html)

	title, chapter, err := s.Provider.GetTitleAndChapter(s.CurrSubUrl)
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

	url, err := s.Provider.GetThumbnail(strconv.Itoa(mangaId))
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
	imgList, err := s.Provider.GetImageList(html)
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
			name := filepath.Base(url)
			s.Mutex.Lock()
			s.ImageBuffers[name] = buf
			s.Mutex.Unlock()
			images[i] = view.Image{Path: name, Index: i}
			wg.Done()
		}(i, url, &wg)
	}

	wg.Wait()
	return images, nil
}

//go:embed favicon.ico
var ico []byte

func (s *Server) AddIco() {
	buf := bytes.NewBuffer(ico)
	s.Mutex.Lock()
	s.ImageBuffers["favicon.ico"] = buf
	s.Mutex.Unlock()
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