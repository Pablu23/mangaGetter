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
	"github.com/rs/zerolog/log"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

func (s *Server) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	s.UpdateMangaList()
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	if s.options.Auth.Enabled {
		auth := s.options.Auth.Get()
		secret := r.PostFormValue("secret")
		http.SetCookie(w, &http.Cookie{
			Name:     "auth",
			Value:    secret,
			Path:     "/",
			MaxAge:   auth.MaxAge,
			Secure:   auth.Secure,
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Login))
	tmpl.Execute(w, nil)
}

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

	http.Redirect(w, r, "/current/", http.StatusFound)
}

func (s *Server) HandleMenu(w http.ResponseWriter, _ *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Menu))
	var all []*database.Manga
	_ = s.DbMgr.Db.Preload("Chapters").Find(&all)
	l := len(all)
	mangaViewModels := make([]view.MangaViewModel, l)
	counter := 0

	var tmp []database.Setting
	s.DbMgr.Db.Find(&tmp)
	settings := make(map[string]database.Setting)
	for _, m := range tmp {
		settings[m.Name] = m
	}

	//TODO: Change all this to be more performant
	for _, manga := range all {
		title := cases.Title(language.English, cases.Compact).String(strings.Replace(manga.Title, "-", " ", -1))

		thumbnail, updated, err := s.LoadThumbnail(manga)
		//TODO: Add default picture instead of not showing Manga at all
		if err != nil {
			continue
		}
		if updated {
			s.DbMgr.Db.Save(manga)
		}
		// This is very slow
		// TODO: put this into own Method
		if manga.LastChapterNum == "" {
			err, updated := s.UpdateLatestAvailableChapter(manga)
			if err != nil {
				log.Error().Err(err).Msg("Could not update latest available chapters")
			}
			if updated {
				s.DbMgr.Db.Save(manga)
			}
		}

		latestChapter, ok := manga.GetLatestChapter()
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

	menuViewModel := view.MenuViewModel{
		Settings: settings,
		Mangas:   mangaViewModels,
	}

	err := tmpl.Execute(w, menuViewModel)
	if err != nil {
		log.Error().Err(err).Msg("Could not template Menu")
	}
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	mangaStr := r.PostFormValue("mangaId")

	if mangaStr == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	mangaId, err := strconv.Atoi(mangaStr)
	if err != nil {
		log.Error().Err(err).Str("Id", mangaStr).Msg("Could not convert id to int")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	s.DbMgr.Delete(mangaId)

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) HandleExit(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusFound)

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
		log.Info().Msg("Cleaned up images")
	}()
}

func (s *Server) HandleCurrent(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(view.GetViewTemplate(view.Viewer))
	mangaId, chapterId, err := s.Provider.GetTitleIdAndChapterId(s.CurrSubUrl)
	if err != nil {
		log.Error().Err(err).Str("subUrl", s.CurrSubUrl).Msg("Could not get TitleId and ChapterId")
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	title, chapterName, err := s.Provider.GetTitleAndChapter(s.CurrSubUrl)
	if err != nil {
		log.Warn().Err(err).Str("subUrl", s.CurrSubUrl).Msg("Could not get Title and Chapter")
	}

	var manga database.Manga
	result := s.DbMgr.Db.First(&manga, mangaId)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		manga = database.NewManga(mangaId, title, time.Now().Unix())
	} else {
		manga.TimeStampUnix = time.Now().Unix()
	}

	var chapter database.Chapter
	result = s.DbMgr.Db.First(&chapter, chapterId)
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		chapterNumberStr := strings.Replace(chapterName, "ch_", "", 1)
		chapter = database.NewChapter(chapterId, mangaId, s.CurrSubUrl, chapterName, chapterNumberStr, time.Now().Unix())
	} else {
		chapter.TimeStampUnix = time.Now().Unix()
	}

	s.DbMgr.Db.Save(&manga)
	s.DbMgr.Db.Save(&chapter)

	err = tmpl.Execute(w, s.CurrViewModel)
	if err != nil {
		log.Error().Err(err).Msg("Could not template Current")
	}
}

func (s *Server) HandleImage(w http.ResponseWriter, r *http.Request) {
	u := r.PathValue("url")
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	buf := s.ImageBuffers[u]
	if buf == nil {
		log.Warn().Str("url", u).Msg("Image not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	_, err := w.Write(buf)
	if err != nil {
		log.Error().Err(err).Msg("Could not write image")
	}
}

//go:embed favicon.ico
var ico []byte

func (s *Server) HandleFavicon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/webp")
	_, err := w.Write(ico)
	if err != nil {
		log.Error().Err(err).Msg("Could not write favicon")
	}
}

func (s *Server) HandleNext(w http.ResponseWriter, r *http.Request) {
	if s.PrevViewModel != nil {
		go func(viewModel view.ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			log.Debug().Msg("Cleaned imagebuffer")
		}(*s.PrevViewModel, s)
	}

	if s.NextViewModel == nil || s.NextSubUrl == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	s.PrevViewModel = s.CurrViewModel
	s.CurrViewModel = s.NextViewModel
	s.PrevSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.NextSubUrl

	go s.LoadNext()

	http.Redirect(w, r, "/current/", http.StatusFound)
}

func (s *Server) HandlePrev(w http.ResponseWriter, r *http.Request) {
	if s.NextViewModel != nil {
		go func(viewModel view.ImageViewModel, s *Server) {
			s.Mutex.Lock()
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			s.Mutex.Unlock()
			log.Debug().Msg("Cleaned imagebuffer")
		}(*s.NextViewModel, s)
	}

	if s.PrevViewModel == nil || s.PrevSubUrl == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	s.NextViewModel = s.CurrViewModel
	s.CurrViewModel = s.PrevViewModel
	s.NextSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.PrevSubUrl

	go s.LoadPrev()

	http.Redirect(w, r, "/current/", http.StatusFound)
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

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) HandleSetting(w http.ResponseWriter, r *http.Request) {
	settingName := r.PostFormValue("setting")
	settingValue := r.PostFormValue(settingName)

	var setting database.Setting
	res := s.DbMgr.Db.First(&setting, "name = ?", settingName)

	if res.Error != nil && errors.Is(res.Error, gorm.ErrRecordNotFound) {
		set := database.NewSetting(settingName, settingValue)
		s.DbMgr.Db.Save(&set)
	} else if res.Error != nil {
		log.Error().Err(res.Error).Send()
	} else {
		s.DbMgr.Db.Model(&setting).Update("value", settingValue)
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) HandleNewQuery(w http.ResponseWriter, r *http.Request) {
	sub := r.PostFormValue("subUrl")

	sub = s.Provider.CleanUrlToSub(sub)
	url := fmt.Sprintf("/title/%s", sub)

	s.CurrSubUrl = url
	s.PrevSubUrl = ""
	s.NextSubUrl = ""
	s.LoadCurr()

	go s.LoadNext()
	go s.LoadPrev()

	http.Redirect(w, r, "/current/", http.StatusFound)
}
