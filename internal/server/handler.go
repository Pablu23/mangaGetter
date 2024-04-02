package server

import (
	"cmp"
	_ "embed"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"html/template"
	"mangaGetter/internal/database"
	"mangaGetter/internal/view"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

func (s *Server) HandleNew(w http.ResponseWriter, r *http.Request) {
	title := r.PathValue("title")
	chapter := r.PathValue("chapter")

	url := fmt.Sprintf("/title/%s/%s", title, chapter)

	s.CurrSubUrl = url
	s.PrevSubUrl = ""
	s.NextSubUrl = ""
	s.LoadCurr()

	go s.LoadNext()
	go s.LoadPrev()

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleMenu(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Menu))
	all := s.DbMgr.Mangas.All()
	l := len(all)
	mangaViewModels := make([]view.MangaViewModel, l)
	counter := 0

	n := time.Now().UnixNano()

	var thumbNs int64 = 0
	var titNs int64 = 0

	//TODO: Change all this to be more performant
	for _, manga := range all {
		title := cases.Title(language.English, cases.Compact).String(strings.Replace(manga.Title, "-", " ", -1))

		t1 := time.Now().UnixNano()

		thumbnail, updated, err := s.LoadThumbnail(&manga)
		//TODO: Add default picture instead of not showing Manga at all
		if err != nil {
			continue
		}
		if updated {
			s.DbMgr.Mangas.Set(manga.Id, manga)
		}

		t2 := time.Now().UnixNano()

		thumbNs += t2 - t1

		t1 = time.Now().UnixNano()

		// This is very slow
		// TODO: put this into own Method
		if manga.LastChapterNum == 0 {
			err, updated := s.UpdateLatestAvailableChapter(&manga)
			if err != nil {
				fmt.Println(err)
			}
			if updated {
				s.DbMgr.Mangas.Set(manga.Id, manga)
			}
		}

		t2 = time.Now().UnixNano()

		titNs += t2 - t1

		latestChapter, ok := manga.GetLatestChapter(&s.DbMgr.Chapters)
		if !ok {
			continue
		}

		mangaViewModels[counter] = view.MangaViewModel{
			ID:         manga.Id,
			Title:      title,
			Number:     latestChapter.Number,
			LastNumber: manga.LastChapterNum,
			// I Hate this time Format... 15 = hh, 04 = mm, 02 = DD, 01 = MM, 06 == YY
			LastTime:     time.Unix(manga.TimeStampUnix, 0).Format("15:04 (02-01-06)"),
			Url:          latestChapter.Url,
			ThumbnailUrl: thumbnail,
		}
		counter++
	}

	fmt.Printf("Loading Thumbnails took %d ms\n", (thumbNs)/1000000)
	fmt.Printf("Loading latest Chapter took %d ms\n", (titNs)/1000000)

	nex := time.Now().UnixNano()
	fmt.Printf("Creating Viewmodels took %d ms\n", (nex-n)/1000000)

	n = time.Now().UnixNano()

	sort := r.URL.Query().Get("sort")

	if sort == "" || sort == "title" {
		slices.SortStableFunc(mangaViewModels, func(a, b view.MangaViewModel) int {
			return cmp.Compare(a.Title, b.Title)
		})
	} else if sort == "chapter" {
		slices.SortStableFunc(mangaViewModels, func(a, b view.MangaViewModel) int {
			return cmp.Compare(b.Number, a.Number)
		})
	} else if sort == "last" {
		slices.SortStableFunc(mangaViewModels, func(a, b view.MangaViewModel) int {
			aT, err := time.Parse("15:04 (02-01-06)", a.LastTime)
			if err != nil {
				return cmp.Compare(a.Title, b.Title)
			}
			bT, err := time.Parse("15:04 (02-01-06)", b.LastTime)
			if err != nil {
				return cmp.Compare(a.Title, b.Title)
			}
			return bT.Compare(aT)
		})
	}

	nex = time.Now().UnixNano()
	fmt.Printf("Sorting took %d ms\n", (nex-n)/1000000)

	menuViewModel := view.MenuViewModel{
		Mangas: mangaViewModels,
	}

	err := tmpl.Execute(w, menuViewModel)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	mangaStr := r.PostFormValue("mangaId")

	if mangaStr == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	mangaId, err := strconv.Atoi(mangaStr)
	if err != nil {
		fmt.Println(err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	err = s.DbMgr.Delete(mangaId)
	if err != nil {
		fmt.Println(err)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleExit(w http.ResponseWriter, r *http.Request) {
	err := s.DbMgr.Save()
	if err != nil {
		fmt.Println(err)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)

	go func() {
		s.Mutex.Lock()
		if s.PrevViewModel != nil {
			for _, img := range s.PrevViewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
		}
		if s.CurrViewModel != nil {

			for _, img := range s.CurrViewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
		}
		if s.NextViewModel != nil {

			for _, img := range s.NextViewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
		}
		s.Mutex.Unlock()
		fmt.Println("Cleaned last Manga")
	}()
}

func (s *Server) HandleCurrent(w http.ResponseWriter, _ *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Viewer))
	mangaId, chapterId, err := s.Provider.GetTitleIdAndChapterId(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
	} else {
		title, chapterName, err := s.Provider.GetTitleAndChapter(s.CurrSubUrl)
		if err != nil {
			fmt.Println(err)
		} else {
			manga, ok := s.DbMgr.Mangas.Get(mangaId)
			if !ok {
				manga = database.NewManga(mangaId, title, time.Now().Unix())
			} else {
				manga.TimeStampUnix = time.Now().Unix()
			}

			chapter, ok := s.DbMgr.Chapters.Get(chapterId)
			if !ok {
				chapterNumberStr := strings.Replace(chapterName, "ch_", "", 1)
				number, err := strconv.Atoi(chapterNumberStr)
				if err != nil {
					fmt.Println(err)
					number = 0
				}

				chapter = database.NewChapter(chapterId, manga.Id, s.CurrSubUrl, chapterName, number, time.Now().Unix())
			} else {
				chapter.TimeStampUnix = time.Now().Unix()
			}

			s.DbMgr.Chapters.Set(chapterId, chapter)
			s.DbMgr.Mangas.Set(mangaId, manga)
		}
	}

	err = tmpl.Execute(w, s.CurrViewModel)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) HandleImage(w http.ResponseWriter, r *http.Request) {
	u := r.PathValue("url")
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
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
}

//go:embed favicon.ico
var ico []byte

func (s *Server) HandleFavicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/webp")
	_, err := w.Write(ico)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) HandleNext(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received Next")

	if s.PrevViewModel != nil {
		go func(viewModel view.ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			fmt.Println("Cleaned out of scope Last")
		}(*s.PrevViewModel, s)
	}

	if s.NextViewModel == nil || s.NextSubUrl == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		err := s.DbMgr.Save()
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	s.PrevViewModel = s.CurrViewModel
	s.CurrViewModel = s.NextViewModel
	s.PrevSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.NextSubUrl

	go s.LoadNext()

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}
func (s *Server) HandlePrev(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received Prev")
	if s.NextViewModel != nil {
		go func(viewModel view.ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			fmt.Println("Cleaned out of scope Last")
		}(*s.NextViewModel, s)
	}

	if s.PrevViewModel == nil || s.PrevSubUrl == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		err := s.DbMgr.Save()
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	s.NextViewModel = s.CurrViewModel
	s.CurrViewModel = s.PrevViewModel
	s.NextSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.PrevSubUrl

	go s.LoadPrev()

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleNewQuery(w http.ResponseWriter, r *http.Request) {
	sub := r.PostFormValue("subUrl")

	url := fmt.Sprintf("/title/%s", sub)

	s.CurrSubUrl = url
	s.PrevSubUrl = ""
	s.NextSubUrl = ""
	s.LoadCurr()

	go s.LoadNext()
	go s.LoadPrev()

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}
