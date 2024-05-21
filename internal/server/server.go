package server

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/pablu23/mangaGetter/internal/database"
	"github.com/pablu23/mangaGetter/internal/provider"
	"github.com/pablu23/mangaGetter/internal/view"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	PrevViewModel *view.ImageViewModel
	CurrViewModel *view.ImageViewModel
	NextViewModel *view.ImageViewModel

	ImageBuffers map[string][]byte
	Mutex        *sync.Mutex

	NextSubUrl string
	CurrSubUrl string
	PrevSubUrl string

	Provider provider.Provider

	IsFirst bool
	IsLast  bool

	DbMgr *database.Manager

	secret string
	mux    *http.ServeMux
}

func New(provider provider.Provider, db *database.Manager, mux *http.ServeMux, secret string) *Server {
	s := Server{
		ImageBuffers: make(map[string][]byte),
		Provider:     provider,
		DbMgr:        db,
		Mutex:        &sync.Mutex{},
		mux:          mux,
		secret:       secret,
	}

	return &s
}

func (s *Server) Start(port int) error {
  s.mux.HandleFunc("GET /login", s.HandleLogin)
  s.mux.HandleFunc("POST /login", s.HandleLoginPost)
	s.mux.HandleFunc("/", s.HandleMenu)
	s.mux.HandleFunc("/new/", s.HandleNewQuery)
	s.mux.HandleFunc("/new/title/{title}/{chapter}", s.HandleNew)
	s.mux.HandleFunc("/current/", s.HandleCurrent)
	s.mux.HandleFunc("/img/{url}/", s.HandleImage)
	s.mux.HandleFunc("POST /next", s.HandleNext)
	s.mux.HandleFunc("POST /prev", s.HandlePrev)
	s.mux.HandleFunc("POST /exit", s.HandleExit)
	s.mux.HandleFunc("POST /delete", s.HandleDelete)
	s.mux.HandleFunc("/favicon.ico", s.HandleFavicon)
	s.mux.HandleFunc("POST /setting/", s.HandleSetting)
	s.mux.HandleFunc("GET /setting/set/{setting}/{value}", s.HandleSettingSet)

	// Update Latest Chapters every 5 Minutes
	go func(s *Server) {
		time.AfterFunc(time.Second*10, func() {
			var all []*database.Manga
			s.DbMgr.Db.Find(&all)
			for _, m := range all {
				err, updated := s.UpdateLatestAvailableChapter(m)
				if err != nil {
					fmt.Println(err)
				}
				if updated {
					s.DbMgr.Db.Save(m)
				}
			}
		})

		for {
			select {
			case <-time.After(time.Minute * 5):
				var all []*database.Manga
				s.DbMgr.Db.Find(&all)
				for _, m := range all {
					err, updated := s.UpdateLatestAvailableChapter(m)
					if err != nil {
						fmt.Println(err)
					}
					if updated {
						s.DbMgr.Db.Save(m)
					}
				}
			}
		}
	}(s)

	fmt.Println("Server starting...")

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.Auth(s.mux),
	}
	return server.ListenAndServe()
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

func (s *Server) UpdateLatestAvailableChapter(manga *database.Manga) (error, bool) {
	fmt.Printf("Updating Manga: %s\n", manga.Title)

	l, err := s.Provider.GetChapterList("/title/" + strconv.Itoa(manga.Id))
	if err != nil {
		return err, false
	}

	le := len(l)
	_, c, err := s.Provider.GetTitleAndChapter(l[le-1])
	if err != nil {
		return err, false
	}

	chapterNumberStr := strings.Replace(c, "ch_", "", 1)

	if manga.LastChapterNum == chapterNumberStr {
		return nil, false
	} else {
		manga.LastChapterNum = chapterNumberStr
		return nil, true
	}
}

func (s *Server) LoadThumbnail(manga *database.Manga) (path string, updated bool, err error) {
	strId := strconv.Itoa(manga.Id)

	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if s.ImageBuffers[strId] != nil {
		return strId, false, nil
	}

	if manga.Thumbnail != nil {
		s.ImageBuffers[strId] = manga.Thumbnail
		return strId, false, nil
	}

	url, err := s.Provider.GetThumbnail(strId)
	if err != nil {
		return "", false, err
	}
	ram, err := addFileToRam(url)
	if err != nil {
		return "", false, err
	}
	manga.Thumbnail = ram
	s.ImageBuffers[strId] = ram
	return strId, true, nil
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

func addFileToRam(url string) ([]byte, error) {
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
	return buf.Bytes(), err
}
