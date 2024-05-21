package server

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/pablu23/mangaGetter/internal/database"
	"github.com/pablu23/mangaGetter/internal/provider"
	"github.com/pablu23/mangaGetter/internal/view"
)

type Server struct {
	ImageBuffers map[string][]byte
	Provider     provider.Provider
	DbMgr        *database.Manager

	Mutex *sync.RWMutex

	Sessions map[string]*UserSession
}

type UserSession struct {
	User database.User

	// Mutex *sync.Mutex

	PrevSubUrl string
	CurrSubUrl string
	NextSubUrl string

	PrevViewModel *view.ImageViewModel
	CurrViewModel *view.ImageViewModel
	NextViewModel *view.ImageViewModel
}

func New(provider provider.Provider, db *database.Manager) *Server {
	s := Server{
		ImageBuffers: make(map[string][]byte),
		Sessions:     make(map[string]*UserSession),
		Provider:     provider,
		DbMgr:        db,
		Mutex:        &sync.RWMutex{},
	}

	return &s
}

func (s *Server) Start(port int) error {
	http.HandleFunc("/register", s.HandleRegister)
	http.HandleFunc("/login", s.HandleLogin)
	http.HandleFunc("/", s.HandleMenu)
	http.HandleFunc("/new/", s.HandleNewQuery)
	http.HandleFunc("/new/title/{title}/{chapter}", s.HandleNew)
	http.HandleFunc("/current/", s.HandleCurrent)
	http.HandleFunc("/img/{url}/", s.HandleImage)
	http.HandleFunc("POST /next", s.HandleNext)
	http.HandleFunc("POST /prev", s.HandlePrev)
	http.HandleFunc("POST /exit", s.HandleExit)
	http.HandleFunc("POST /delete", s.HandleDelete)
	http.HandleFunc("/favicon.ico", s.HandleFavicon)
	http.HandleFunc("POST /setting/", s.HandleSetting)
	http.HandleFunc("GET /setting/set/{setting}/{value}", s.HandleSettingSet)

	// Update Latest Chapters every 5 Minutes
	// go func(s *Server) {
	// 	time.AfterFunc(time.Second*10, func() {
	// 		var all []*database.Manga
	// 		s.DbMgr.Db.Find(&all)
	// 		for _, m := range all {
	// 			err, updated := s.UpdateLatestAvailableChapter(m)
	// 			if err != nil {
	// 				fmt.Println(err)
	// 			}
	// 			if updated {
	// 				s.DbMgr.Db.Save(m)
	// 			}
	// 		}
	// 	})
	//
	// 	for {
	// 		select {
	// 		case <-time.After(time.Minute * 5):
	// 			var all []*database.Manga
	// 			s.DbMgr.Db.Find(&all)
	// 			for _, m := range all {
	// 				err, updated := s.UpdateLatestAvailableChapter(m)
	// 				if err != nil {
	// 					fmt.Println(err)
	// 				}
	// 				if updated {
	// 					s.DbMgr.Db.Save(m)
	// 				}
	// 			}
	// 		}
	// 	}
	// }(s)
	//
	fmt.Println("Server starting...")
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	return err
}

func (s *Server) LoadNext(session *UserSession) {
	next, err := s.Provider.GetHtml(session.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		session.NextSubUrl = ""
		session.NextViewModel = nil
		return
	}

	html, err := s.Provider.GetHtml(next)
	if err != nil {
		fmt.Println(err)
		session.NextSubUrl = ""
		session.NextViewModel = nil
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		fmt.Println(err)
		session.NextSubUrl = ""
		session.NextViewModel = nil
		return
	}

	title, chapter, err := s.Provider.GetTitleAndChapter(next)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	session.NextViewModel = &view.ImageViewModel{Images: imagesNext, Title: full}
	session.NextSubUrl = next
	fmt.Println("Loaded next")
}

func (s *Server) LoadPrev(session *UserSession) {
	c, err := s.Provider.GetHtml(session.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		session.PrevSubUrl = ""
		session.PrevViewModel = nil
		return
	}
	prev, err := s.Provider.GetPrev(c)
	if err != nil {
		fmt.Println(err)
		session.PrevSubUrl = ""
		session.PrevViewModel = nil
		return
	}
	html, err := s.Provider.GetHtml(prev)
	if err != nil {
		fmt.Println(err)
		session.PrevSubUrl = ""
		session.PrevViewModel = nil
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		fmt.Println(err)
		session.PrevSubUrl = ""
		session.PrevViewModel = nil
		return
	}

	title, chapter, err := s.Provider.GetTitleAndChapter(prev)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	session.PrevViewModel = &view.ImageViewModel{Images: imagesNext, Title: full}

	session.PrevSubUrl = prev
	fmt.Println("Loaded prev")
}

func (s *Server) LoadCurr(session *UserSession) {
	html, err := s.Provider.GetHtml(session.CurrSubUrl)
	if err != nil {
		panic(err)
	}

	imagesCurr, err := s.AppendImagesToBuf(html)

	title, chapter, err := s.Provider.GetTitleAndChapter(session.CurrSubUrl)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	session.CurrViewModel = &view.ImageViewModel{Images: imagesCurr, Title: full}
	fmt.Println("Loaded current")
}

func (s *Server) UpdateLatestAvailableChapter(manga *database.Manga) (error, bool) {
	fmt.Printf("Updating Manga: %s\n", manga.Definition.Title)

	l, err := s.Provider.GetChapterList("/title/" + strconv.Itoa(manga.Definition.Id))
	if err != nil {
		return err, false
	}

	le := len(l)
	_, c, err := s.Provider.GetTitleAndChapter(l[le-1])
	if err != nil {
		return err, false
	}

	chapterNumberStr := strings.Replace(c, "ch_", "", 1)

	if manga.Definition.LastChapterNum == chapterNumberStr {
		return nil, false
	} else {
		manga.Definition.LastChapterNum = chapterNumberStr
		return nil, true
	}
}

func (s *Server) LoadThumbnail(manga *database.Manga) (path string, updated bool, err error) {
	strId := strconv.Itoa(manga.Definition.Id)

	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if s.ImageBuffers[strId] != nil {
		return strId, false, nil
	}

	if manga.Definition.Thumbnail != nil {
		s.ImageBuffers[strId] = manga.Definition.Thumbnail
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
	manga.Definition.Thumbnail = ram
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
