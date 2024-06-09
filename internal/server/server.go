package server

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pablu23/mangaGetter/internal/database"
	"github.com/pablu23/mangaGetter/internal/provider"
	"github.com/pablu23/mangaGetter/internal/view"
	"github.com/rs/zerolog/log"
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

	mux *http.ServeMux

	options Options
	secret  string
}

func New(provider provider.Provider, db *database.Manager, mux *http.ServeMux, options ...func(*Options)) *Server {
	opts := NewDefaultOptions()
	for _, opt := range options {
		opt(&opts)
	}

	s := Server{
		ImageBuffers: make(map[string][]byte),
		Provider:     provider,
		DbMgr:        db,
		Mutex:        &sync.Mutex{},
		mux:          mux,
		options:      opts,
	}

	return &s
}

func (s *Server) RegisterRoutes() {
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
	s.mux.HandleFunc("GET /update", s.HandleUpdate)
	s.mux.HandleFunc("POST /disable", s.HandleDisable)
	s.mux.HandleFunc("GET /archive", s.HandleArchive)
}

func (s *Server) Start() error {
	server := http.Server{
		Addr:    fmt.Sprintf(":%d", s.options.Port),
		Handler: s.mux,
	}
	s.RegisterRoutes()
	s.registerUpdater()

	if s.options.Auth.Enabled {
		auth := s.options.Auth.Get()
		switch auth.LoadType {
		case Raw:
			s.secret = auth.Secret
		case File:
			secretBytes, err := os.ReadFile(auth.Secret)
			if err != nil {
				return err
			}
			s.secret = string(secretBytes)
		}
		s.secret = strings.TrimSpace(s.secret)
		server.Handler = s.Auth(s.mux)
	}

	if s.options.Tls.Enabled {
		tlsOpts := s.options.Tls.Get()
		server.TLSConfig = &tls.Config{
			GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
				cert, err := tls.LoadX509KeyPair(tlsOpts.CertPath, tlsOpts.KeyPath)
				if err != nil {
					return nil, err
				}
				return &cert, err
			},
		}
		log.Info().Int("Port", s.options.Port).Str("Cert", tlsOpts.CertPath).Str("Key", tlsOpts.KeyPath).Msg("Starting server")
		return server.ListenAndServeTLS("", "")
	} else {
		log.Info().Int("Port", s.options.Port).Msg("Starting server")
		return server.ListenAndServe()
	}
}

func (s *Server) UpdateMangaList() {
	var all []*database.Manga
	s.DbMgr.Db.Where("enabled = 1").Find(&all)
	for _, m := range all {
		err, updated := s.UpdateLatestAvailableChapter(m)
		if err != nil {
			log.Error().Err(err).Str("Manga", m.Title).Msg("Could not update latest available chapters")
		}
		if updated {
			s.DbMgr.Db.Save(m)
		}
	}
}

func (s *Server) registerUpdater() {
	if s.options.UpdateInterval > 0 {
		log.Info().Str("Interval", s.options.UpdateInterval.String()).Msg("Registering Updater")
		go func(s *Server) {
			for {
				select {
				case <-time.After(s.options.UpdateInterval):
					s.UpdateMangaList()
				}
			}
		}(s)
	}
}

func (s *Server) LoadNext() {
	c, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		log.Error().Err(err).Msg("Could not get Html for current chapter")
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	next, err := s.Provider.GetNext(c)
	if err != nil {
		log.Error().Err(err).Msg("Could not load next chapter")
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	html, err := s.Provider.GetHtml(next)
	if err != nil {
		log.Error().Err(err).Msg("Could not get Html for next chapter")
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		log.Error().Err(err).Msg("Could not download images")
		s.NextSubUrl = ""
		s.NextViewModel = nil
		return
	}

	title, chapter, err := s.Provider.GetTitleAndChapter(next)
	if err != nil {
		log.Warn().Err(err).Str("Url", next).Msg("Could not extract title and chapter")
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.NextViewModel = &view.ImageViewModel{Images: imagesNext, Title: full}
	s.NextSubUrl = next
	log.Debug().Msg("Successfully loaded next chapter")
}

func (s *Server) LoadPrev() {
	c, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		log.Error().Err(err).Msg("Could not get Html for current chapter")
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}
	prev, err := s.Provider.GetPrev(c)
	if err != nil {
		log.Error().Err(err).Msg("Could not load prev chapter")
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}
	html, err := s.Provider.GetHtml(prev)
	if err != nil {
		log.Error().Err(err).Msg("Could not get Html for prev chapter")
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}

	imagesNext, err := s.AppendImagesToBuf(html)
	if err != nil {
		log.Error().Err(err).Msg("Could not download images")
		s.PrevSubUrl = ""
		s.PrevViewModel = nil
		return
	}

	title, chapter, err := s.Provider.GetTitleAndChapter(prev)
	if err != nil {
		log.Warn().Err(err).Str("Url", prev).Msg("Could not extract title and chapter")
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.PrevViewModel = &view.ImageViewModel{Images: imagesNext, Title: full}

	s.PrevSubUrl = prev
	log.Debug().Msg("Successfully loaded prev chapter")

}

func (s *Server) LoadCurr() {
	html, err := s.Provider.GetHtml(s.CurrSubUrl)
	if err != nil {
		log.Error().Err(err).Msg("Could not get Html for current chapter")
		s.NextSubUrl = ""
		s.PrevSubUrl = ""
		s.CurrSubUrl = ""
		s.NextViewModel = nil
		s.CurrViewModel = nil
		s.PrevViewModel = nil
		return
	}

	imagesCurr, err := s.AppendImagesToBuf(html)

	title, chapter, err := s.Provider.GetTitleAndChapter(s.CurrSubUrl)
	if err != nil {
		log.Warn().Err(err).Str("Url", s.CurrSubUrl).Msg("Could not extract title and chapter")
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.CurrViewModel = &view.ImageViewModel{Images: imagesCurr, Title: full}
	log.Debug().Msg("Successfully loaded curr chapter")
}

func (s *Server) UpdateLatestAvailableChapter(manga *database.Manga) (error, bool) {
	log.Info().Str("Manga", manga.Title).Msg("Updating Manga")

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
			log.Error().Err(err).Msg("Could not close http body")
		}
	}(resp.Body)

	buf := new(bytes.Buffer)

	// Write the body to file
	_, err = io.Copy(buf, resp.Body)
	return buf.Bytes(), err
}
