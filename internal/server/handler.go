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

func (s *Server) HandleMenu(w http.ResponseWriter, _ *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Menu))

	fmt.Println("Locking Rw in handler.go:43")
	s.DbMgr.Rw.Lock()
	defer func() {
		fmt.Println("Unlocking Rw in handler.go:46")
		s.DbMgr.Rw.Unlock()
	}()

	all := s.DbMgr.Mangas
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

		thumbnail, err := s.LoadThumbnail(manga)
		//TODO: Add default picture instead of not showing Manga at all
		if err != nil {
			continue
		}
		manga.Thumbnail = s.ImageBuffers[thumbnail]

		t2 := time.Now().UnixNano()

		thumbNs += t2 - t1

		t1 = time.Now().UnixNano()

		// This is very slow
		// TODO: put this into own Method
		if manga.LastChapterNum == 0 {
			l, err := s.Provider.GetChapterList("/title/" + strconv.Itoa(manga.Id))
			if err != nil {
				continue
			}

			le := len(l)
			_, c, err := s.Provider.GetTitleAndChapter(l[le-1])
			if err != nil {
				continue
			}

			chapterNumberStr := strings.Replace(c, "ch_", "", 1)

			i, err := strconv.Atoi(chapterNumberStr)
			if err != nil {
				continue
			}

			manga.LastChapterNum = i
		}

		t2 = time.Now().UnixNano()

		titNs += t2 - t1

		mangaViewModels[counter] = view.MangaViewModel{
			ID:         manga.Id,
			Title:      title,
			Number:     manga.LatestChapter.Number,
			LastNumber: manga.LastChapterNum,
			// I Hate this time Format... 15 = hh, 04 = mm, 02 = DD, 01 = MM, 06 == YY
			LastTime:     time.Unix(manga.TimeStampUnix, 0).Format("15:04 (02-01-06)"),
			Url:          manga.LatestChapter.Url,
			ThumbnailUrl: thumbnail,
		}
		counter++
	}

	fmt.Printf("Loading Thumbnails took %d ms\n", (thumbNs)/1000000)
	fmt.Printf("Loading latest Chapter took %d ms\n", (titNs)/1000000)

	nex := time.Now().UnixNano()
	fmt.Printf("Creating Viewmodels took %d ms\n", (nex-n)/1000000)

	n = time.Now().UnixNano()

	slices.SortStableFunc(mangaViewModels, func(a, b view.MangaViewModel) int {
		return cmp.Compare(a.Title, b.Title)
	})

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

	fmt.Println("Locking Rw in handler.go:125")
	s.DbMgr.Rw.Lock()
	defer func() {
		fmt.Println("Unlocking Rw in handler.go:128")
		s.DbMgr.Rw.Unlock()
	}()

	mangaId, chapterId, err := s.Provider.GetTitleIdAndChapterId(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
	} else {
		title, chapter, err := s.Provider.GetTitleAndChapter(s.CurrSubUrl)
		if err != nil {
			fmt.Println(err)
		} else {
			var manga *database.Manga
			if s.DbMgr.Mangas[mangaId] == nil {
				manga = &database.Manga{
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

				s.DbMgr.Chapters[chapterId] = &database.Chapter{
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

			s.DbMgr.Mangas[mangaId].LatestChapter = s.DbMgr.Chapters[chapterId]
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
