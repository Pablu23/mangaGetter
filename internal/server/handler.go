package server

import (
	"cmp"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pablu23/mangaGetter/internal/database"
	"github.com/pablu23/mangaGetter/internal/view"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

func (s *Server) getSessionFromCookie(w http.ResponseWriter, r *http.Request) (*UserSession, error) {
	cookie, err := r.Cookie("session")
	if err != nil {
		switch {
		case errors.Is(err, http.ErrNoCookie):
			// http.Error(w, "cookie not found", http.StatusBadRequest)
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		default:
			fmt.Println(err)
			http.Error(w, "server error", http.StatusInternalServerError)
		}
	}
	session, ok := s.Sessions[cookie.Value]
	if !ok {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return nil, errors.New("Unknown Session")
	}
	return session, err
}

func (s *Server) HandleRegister(w http.ResponseWriter, r *http.Request) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	admin := database.User{
		Id:          1,
		DisplayName: "admin",
		LoginName:   "admin",
	}
	s.DbMgr.Db.Create(&admin)

	http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	// Login
	s.Sessions["abcd"] = &UserSession{
		User: database.User{
			Id:          1,
			DisplayName: "admin",
			LoginName:   "admin",
		},
	}

	cookie := http.Cookie{
		Name:     "session",
		Value:    "abcd",
		Path:     "/",
		MaxAge:   3600,
		Secure:   true,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleNew(w http.ResponseWriter, r *http.Request) {
	title := r.PathValue("title")
	chapter := r.PathValue("chapter")

	url := fmt.Sprintf("/title/%s/%s", title, chapter)

	session, err := s.getSessionFromCookie(w, r)
	if err != nil {
		return
	}

	session.CurrSubUrl = url
	session.PrevSubUrl = ""
	session.NextSubUrl = ""
	s.LoadCurr(session)

	go s.LoadNext(session)
	go s.LoadPrev(session)

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleMenu(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Menu))

	session, err := s.getSessionFromCookie(w, r)
	if err != nil {
		return
	}
	var all []*database.Manga
	_ = s.DbMgr.Db.Preload("Chapters").Where("user_id = ?", session.User.Id).Find(&all)
	l := len(all)
	mangaViewModels := make([]view.MangaViewModel, l)
	counter := 0

	n := time.Now().UnixNano()

	var tmp []database.Setting
	s.DbMgr.Db.Find(&tmp)
	settings := make(map[string]database.Setting)
	for _, m := range tmp {
		settings[m.Name] = m
	}

	var thumbNs int64 = 0
	var titNs int64 = 0

	//TODO: Change all this to be more performant
	for _, manga := range all {
		title := cases.Title(language.English, cases.Compact).String(strings.Replace(manga.Definition.Title, "-", " ", -1))

		t1 := time.Now().UnixNano()

		thumbnail, updated, err := s.LoadThumbnail(manga)
		//TODO: Add default picture instead of not showing Manga at all
		if err != nil {
			continue
		}
		if updated {
			s.DbMgr.Db.Save(manga)
		}

		t2 := time.Now().UnixNano()

		thumbNs += t2 - t1

		t1 = time.Now().UnixNano()

		// This is very slow
		// TODO: put this into own Method
		if manga.Definition.LastChapterNum == "" {
			err, updated := s.UpdateLatestAvailableChapter(manga)
			if err != nil {
				fmt.Println(err)
			}
			if updated {
				s.DbMgr.Db.Save(manga.Definition)
			}
		}

		t2 = time.Now().UnixNano()

		titNs += t2 - t1
		latestChapter, ok := manga.GetLatestChapter()
		if !ok {
			continue
		}

		mangaViewModels[counter] = view.MangaViewModel{
			ID:         manga.Definition.Id,
			Title:      title,
			Number:     latestChapter.Number,
			LastNumber: manga.Definition.LastChapterNum,
			// I Hate this time Format... 15 = hh, 04 = mm, 02 = DD, 01 = MM, 06 == YY
			LastTime:     time.Unix(manga.TimeStampUnix, 0).Format("15:04 (02-01-06)"),
			Url:          latestChapter.Url,
			ThumbnailUrl: thumbnail,
		}
		counter++
	}

	fmt.Printf("Loading Thumbnails took %d ms\n", (thumbNs)/1000000)
	fmt.Printf("Loading latest Chapters took %d ms\n", (titNs)/1000000)

	nex := time.Now().UnixNano()
	fmt.Printf("Creating Viewmodels took %d ms\n", (nex-n)/1000000)

	n = time.Now().UnixNano()

	order, ok := settings["order"]
	if !ok || order.Value == "title" {
		slices.SortStableFunc(mangaViewModels, func(a, b view.MangaViewModel) int {
			return cmp.Compare(a.Title, b.Title)
		})
	} else if order.Value == "chapter" {
		slices.SortStableFunc(mangaViewModels, func(a, b view.MangaViewModel) int {
			return cmp.Compare(b.Number, a.Number)
		})
	} else if order.Value == "last" {
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
		Settings: settings,
		Mangas:   mangaViewModels,
	}

	err = tmpl.Execute(w, menuViewModel)
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

	s.DbMgr.Delete(mangaId)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleExit(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)

	// session, err := s.getSessionFromCookie(w, r)
	// if err != nil {
	// 	return
	// }

	go func() {
		// session.Mutex.Lock()
		// if session.PrevViewModel != nil {
		// 	for _, img := range session.PrevViewModel.Images {
		// 		delete(s.ImageBuffers, img.Path)
		// 	}
		// }
		// if session.CurrViewModel != nil {
		//
		// 	for _, img := range session.CurrViewModel.Images {
		// 		delete(s.ImageBuffers, img.Path)
		// 	}
		// }
		// if session.NextViewModel != nil {
		//
		// 	for _, img := range session.NextViewModel.Images {
		// 		delete(s.ImageBuffers, img.Path)
		// 	}
		// }
		// session.Mutex.Unlock()
		fmt.Println("Cleaned last Manga")
	}()
}

func (s *Server) HandleCurrent(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Viewer))
	session, err := s.getSessionFromCookie(w, r)
	if err != nil {
		return
	}
	mangaId, chapterId, err := s.Provider.GetTitleIdAndChapterId(session.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
	}

	title, chapterName, err := s.Provider.GetTitleAndChapter(session.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
	}

	var mangaDef database.MangaDefinition
	result := s.DbMgr.Db.First(&mangaDef, mangaId)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		mangaDef = database.NewMangaDefinition(mangaId, title)
	}

	var manga database.Manga
	result = s.DbMgr.Db.Where("user_id = ?", session.User.Id).First(&manga, "manga_definition_id = ?", mangaId)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		manga = database.NewManga(mangaDef, session.User, time.Now().Unix())
	}

	var chapter database.Chapter
	result = s.DbMgr.Db.Where("user_id = ?", session.User.Id).First(&chapter, "chapter_id = ?", chapterId)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		chapterNumberStr := strings.Replace(chapterName, "ch_", "", 1)
		chapter = database.NewChapter(chapterId, mangaId, session.User.Id, session.CurrSubUrl, chapterName, chapterNumberStr, time.Now().Unix())
	} else {
		chapter.TimeStampUnix = time.Now().Unix()
	}

  s.DbMgr.Db.Save(&mangaDef)
	s.DbMgr.Db.Save(&manga)
	s.DbMgr.Db.Save(&chapter)

	err = tmpl.Execute(w, session.CurrViewModel)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) HandleImage(w http.ResponseWriter, r *http.Request) {
	u := r.PathValue("url")
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	buf := s.ImageBuffers[u]
	if buf == nil {
		fmt.Printf("url: %s is nil\n", u)
		w.WriteHeader(400)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	_, err := w.Write(buf)
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

	session, err := s.getSessionFromCookie(w, r)
	if err != nil {
		return
	}

	if session.PrevViewModel != nil {
		go func(viewModel view.ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			fmt.Println("Cleaned out of scope Last")
		}(*session.PrevViewModel, s)
	}

	if session.NextViewModel == nil || session.NextSubUrl == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	session.PrevViewModel = session.CurrViewModel
	session.CurrViewModel = session.NextViewModel
	session.PrevSubUrl = session.CurrSubUrl
	session.CurrSubUrl = session.NextSubUrl

	go s.LoadNext(session)

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}
func (s *Server) HandlePrev(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received Prev")

	session, err := s.getSessionFromCookie(w, r)
	if err != nil {
		return
	}
	if session.NextViewModel != nil {
		go func(viewModel view.ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			fmt.Println("Cleaned out of scope Last")
		}(*session.NextViewModel, s)
	}

	if session.PrevViewModel == nil || session.PrevSubUrl == "" {
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	session.NextViewModel = session.CurrViewModel
	session.CurrViewModel = session.PrevViewModel
	session.NextSubUrl = session.CurrSubUrl
	session.CurrSubUrl = session.PrevSubUrl

	go s.LoadPrev(session)

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleSettingSet(w http.ResponseWriter, r *http.Request) {
	settingName := r.PathValue("setting")
	settingValue := r.PathValue("value")

	var setting database.Setting
	res := s.DbMgr.Db.First(&setting, "name = ?", settingName)

	if res.Error != nil && errors.Is(res.Error, gorm.ErrRecordNotFound) {
		set := database.NewSetting(settingName, settingValue)
		s.DbMgr.Db.Save(&set)
	} else {
		s.DbMgr.Db.Model(&setting).Update("value", settingValue)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleSetting(w http.ResponseWriter, r *http.Request) {
	settingName := r.PostFormValue("setting")
	settingValue := r.PostFormValue(settingName)

	var setting database.Setting
	res := s.DbMgr.Db.First(&setting, "name = ?", settingName)

	if res.Error != nil && errors.Is(res.Error, gorm.ErrRecordNotFound) {
		set := database.NewSetting(settingName, settingValue)
		s.DbMgr.Db.Save(&set)
	} else {
		s.DbMgr.Db.Model(&setting).Update("value", settingValue)
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleNewQuery(w http.ResponseWriter, r *http.Request) {
	sub := r.PostFormValue("subUrl")

	url := fmt.Sprintf("/title/%s", sub)

	session, err := s.getSessionFromCookie(w, r)
	if err != nil {
		return
	}
	session.CurrSubUrl = url
	session.PrevSubUrl = ""
	session.NextSubUrl = ""
	s.LoadCurr(session)

	go s.LoadNext(session)
	go s.LoadPrev(session)

	http.Redirect(w, r, "/current/", http.StatusTemporaryRedirect)
}
