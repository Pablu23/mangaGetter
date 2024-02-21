package main

import (
	"bytes"
	"fmt"
	"golang.org/x/text/language"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/cases"
)

type Server struct {
	PrevViewModel *ImageViewModel
	CurrViewModel *ImageViewModel
	NextViewModel *ImageViewModel

	ImageBuffers map[string]*bytes.Buffer
	Mutex        *sync.Mutex

	NextSubUrl string
	CurrSubUrl string
	PrevSubUrl string

	Provider Provider

	IsFirst bool
	IsLast  bool

	DbMgr *DatabaseManager

	// I'm not even sure if this helps.
	// If you press next and then prev too fast you still lock yourself out
	NextReady chan bool
	PrevReady chan bool
}

func (s *Server) HandleImage(w http.ResponseWriter, r *http.Request) {
	u := r.PathValue("url")
	s.Mutex.Lock()
	buf := s.ImageBuffers[u]
	if buf == nil {
		fmt.Printf("url: %s is nil\n", u)
		w.WriteHeader(400)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	_, err := w.Write(buf.Bytes())
	if err != nil {
		fmt.Println(err)
	}
	s.Mutex.Unlock()
}

func (s *Server) HandleNext(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received Next")

	if s.PrevViewModel != nil {
		go func(viewModel ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			fmt.Println("Cleaned out of scope Last")
		}(*s.PrevViewModel, s)
	}

	s.PrevViewModel = s.CurrViewModel
	s.CurrViewModel = s.NextViewModel
	s.PrevSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.NextSubUrl

	<-s.NextReady

	go s.LoadNext()

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}

func (s *Server) LoadNext() {
	c, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		return
	}

	next, err := s.Provider.GetNext(c)
	if err != nil {
		fmt.Println(err)
		return
	}

	html, err := s.Provider.GetHtml(next)
	if err != nil {
		fmt.Println(err)
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		fmt.Println(err)
		return
	}

	title, chapter, err := getTitleAndChapter(next)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.NextViewModel = &ImageViewModel{Images: imagesNext, Title: full}

	s.NextSubUrl = next
	fmt.Println("Loaded next")
	s.NextReady <- true
}

func (s *Server) LoadPrev() {
	c, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		return
	}
	prev, err := s.Provider.GetPrev(c)
	if err != nil {
		fmt.Println(err)
		return
	}
	html, err := s.Provider.GetHtml(prev)
	if err != nil {
		fmt.Println(err)
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		fmt.Println(err)
		return
	}

	title, chapter, err := getTitleAndChapter(prev)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.PrevViewModel = &ImageViewModel{Images: imagesNext, Title: full}

	s.PrevSubUrl = prev
	fmt.Println("Loaded prev")
	s.PrevReady <- true
}

func (s *Server) HandlePrev(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received Prev")
	if s.NextViewModel != nil {
		go func(viewModel ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			fmt.Println("Cleaned out of scope Last")
		}(*s.NextViewModel, s)
	}

	s.NextViewModel = s.CurrViewModel
	s.CurrViewModel = s.PrevViewModel
	s.NextSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.PrevSubUrl

	<-s.PrevReady

	go s.LoadPrev()

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleCurrent(w http.ResponseWriter, _ *http.Request) {
	tmpl := template.Must(template.ParseFiles("viewer.gohtml"))

	s.DbMgr.rw.Lock()
	defer s.DbMgr.rw.Unlock()

	mangaId, chapterId, err := getMangaIdAndChapterId(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
	} else {
		title, chapter, err := getTitleAndChapter(s.CurrSubUrl)
		if err != nil {
			fmt.Println(err)
		} else {
			var manga *Manga
			if s.DbMgr.Mangas[mangaId] == nil {
				manga = &Manga{
					Id:            mangaId,
					Title:         title,
					TimeStampUnix: time.Now().Unix(),
				}
				s.DbMgr.Mangas[mangaId] = manga
			} else {
				manga = s.DbMgr.Mangas[mangaId]
				s.DbMgr.Mangas[mangaId].TimeStampUnix = time.Now().Unix()
			}

			if s.DbMgr.Chapters[chapterId] == nil {
				chapterNumberStr := strings.Replace(chapter, "ch_", "", 1)
				number, err := strconv.Atoi(chapterNumberStr)
				if err != nil {
					fmt.Println(err)
					number = 0
				}

				s.DbMgr.Chapters[chapterId] = &Chapter{
					Id:            chapterId,
					Manga:         manga,
					Url:           s.CurrSubUrl,
					Name:          chapter,
					Number:        number,
					TimeStampUnix: time.Now().Unix(),
				}
			} else {
				s.DbMgr.Chapters[chapterId].TimeStampUnix = time.Now().Unix()
			}
		}
	}

	err = tmpl.Execute(w, s.CurrViewModel)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) HandleNew(w http.ResponseWriter, r *http.Request) {
	title := r.PathValue("title")
	chapter := r.PathValue("chapter")

	url := fmt.Sprintf("/title/%s/%s", title, chapter)

	s.Mutex.Lock()
	s.ImageBuffers = make(map[string]*bytes.Buffer)
	s.Mutex.Unlock()
	s.CurrSubUrl = url
	s.PrevSubUrl = ""
	s.NextSubUrl = ""
	s.LoadCurr()

	go s.LoadNext()
	go s.LoadPrev()

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}

func (s *Server) LoadCurr() {
	html, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		panic(err)
	}

	imagesCurr, err := s.AppendImagesToBuf(html)

	title, chapter, err := getTitleAndChapter(s.CurrSubUrl)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.CurrViewModel = &ImageViewModel{Images: imagesCurr, Title: full}
	fmt.Println("Loaded current")
}

func (s *Server) AppendImagesToBuf(html string) ([]Image, error) {
	imgList, err := s.Provider.GetImageList(html)
	if err != nil {
		return nil, err
	}

	images := make([]Image, len(imgList))

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
			images[i] = Image{Path: name, Index: i}
			wg.Done()
		}(i, url, &wg)
	}

	wg.Wait()
	return images, nil
}

func (s *Server) HandleMenu(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("menu.gohtml"))

	s.DbMgr.rw.Lock()
	defer s.DbMgr.rw.Unlock()

	all := s.DbMgr.Mangas
	l := len(all)
	mangaViewModels := make([]MangaViewModel, l)
	counter := 0

	for _, manga := range all {
		title := cases.Title(language.English, cases.Compact).String(strings.Replace(manga.Title, "-", " ", -1))

		mangaViewModels[counter] = MangaViewModel{
			Title:  title,
			Number: manga.LatestChapter.Number,
			// I Hate this time Format... 15 = hh, 04 = mm, 02 = DD, 01 = MM, 06 == YY
			LastTime: time.Unix(manga.TimeStampUnix, 0).Format("15:04 (02-01-06)"),
			Url:      manga.LatestChapter.Url,
		}
		counter++
	}

	menuViewModel := MenuViewModel{
		Mangas: mangaViewModels,
	}

	err := tmpl.Execute(w, menuViewModel)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) HandleExit(w http.ResponseWriter, r *http.Request) {
	err := s.DbMgr.Save()
	if err != nil {
		fmt.Println(err)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
