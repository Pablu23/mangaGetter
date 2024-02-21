package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type NoMoreError struct {
	Err error
}

func (e *NoMoreError) Error() string {
	return "no more images available"
}

type Image struct {
	Path  string
	Index int
}

type ImageViewModel struct {
	Title  string
	Images []Image
}

type Server struct {
	PrevViewModel *ImageViewModel
	CurrViewModel *ImageViewModel
	NextViewModel *ImageViewModel
	ImageBuffers  map[string]*bytes.Buffer
	NextSubUrl    string
	CurrSubUrl    string
	PrevSubUrl    string

	IsFirst bool
	IsLast  bool

	// I'm not even sure if this helps
	// If you press next and then prev too fast you still lock yourself out
	NextReady chan bool
	PrevReady chan bool
}

func getImageList(html string) ([]string, error) {
	reg, err := regexp.Compile(`<astro-island.*props=".*;imageFiles&quot;:\[1,&quot;\[(.*)]&quot;]`)
	if err != nil {
		return nil, err
	}
	m := reg.FindStringSubmatch(html)

	if len(m) <= 0 {
		return nil, &NoMoreError{Err: errors.New("no more content")}
	}
	match := m[1]

	reg, err = regexp.Compile(`\[0,\\&quot;([^&]*)\\&quot;]`)
	if err != nil {
		return nil, err
	}

	matches := reg.FindAllStringSubmatch(match, -1)
	l := len(matches)
	result := make([]string, l)
	for i, m := range matches {
		result[i] = m[1]
	}

	return result, nil
}

func getHtmlFor(titleSubUrl string) (string, error) {
	url := fmt.Sprintf("https://bato.to%s?load=2", titleSubUrl)
	resp, err := http.Get(url)

	// TODO: Testing for above 300 is dirty
	if err != nil && resp.StatusCode > 300 {
		return "", errors.New("could not get html")
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Could not close body because: %v\n", err)
		}
	}(resp.Body)

	all, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	h := string(all)

	return h, nil
}

func getNext(html string) (subUrl string, err error) {
	reg, err := regexp.Compile(`<a data-hk="0-6-0" .*? href="(.*?)["']`)
	match := reg.FindStringSubmatch(html)

	return match[1], err
}

func getPrev(html string) (subUrl string, err error) {
	reg, err := regexp.Compile(`<a data-hk="0-5-0" .*? href="(.*?)["']`)
	match := reg.FindStringSubmatch(html)

	return match[1], err
}

func appendImagesToBuf(html string, imageBuffs map[string]*bytes.Buffer) ([]Image, error) {
	imgList, err := getImageList(html)
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
			imageBuffs[name] = buf
			images[i] = Image{Path: name, Index: i}
			wg.Done()
		}(i, url, &wg)
	}

	wg.Wait()
	return images, nil
}

func main() {
	curr := "/title/143267-blooming-love/2636103-ch_20"

	server := Server{
		ImageBuffers: make(map[string]*bytes.Buffer),
		CurrSubUrl:   curr,
		NextReady:    make(chan bool),
		PrevReady:    make(chan bool),
	}

	server.loadCurr()
	go server.loadPrev()
	go server.loadNext()

	http.HandleFunc("/", server.HandleCurrent)
	http.HandleFunc("/img/{url}/", server.HandleImage)
	http.HandleFunc("POST /next", server.handleNext)
	http.HandleFunc("POST /prev", server.handlePrev)
	http.HandleFunc("/new/{title}/{chapter}", server.HandleNew)

	fmt.Println("Server running")
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func (s *Server) HandleImage(w http.ResponseWriter, r *http.Request) {
	u := r.PathValue("url")
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

func (s *Server) handleNext(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received Next")

	if s.PrevViewModel != nil {
		go func(viewModel ImageViewModel, s *Server) {
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			fmt.Println("Cleaned out of scope Last")
		}(*s.PrevViewModel, s)
	}

	s.PrevViewModel = s.CurrViewModel
	s.CurrViewModel = s.NextViewModel
	s.PrevSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.NextSubUrl

	<-s.NextReady

	go s.loadNext()

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) loadNext() {
	c, err := getHtmlFor(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		return
	}

	next, err := getNext(c)
	if err != nil {
		fmt.Println(err)
		return
	}

	html, err := getHtmlFor(next)
	if err != nil {
		fmt.Println(err)
		return
	}

	imagesNext, err := appendImagesToBuf(html, s.ImageBuffers)
	//if err != nil && errors.Is(err, &NoMoreError{}) {
	//	fmt.Println(err)
	//	return
	//} else
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

func getTitleAndChapter(url string) (title string, chapter string, err error) {
	reg, err := regexp.Compile(`/title/\d*-(.*?)/\d*-(.*)`)
	if err != nil {
		return "", "", err
	}

	matches := reg.FindAllStringSubmatch(url, -1)
	if len(matches) <= 0 {
		return "", "", errors.New("no title or chapter found")
	}

	return matches[0][1], matches[0][2], nil
}

func (s *Server) loadPrev() {
	c, err := getHtmlFor(s.CurrSubUrl)
	if err != nil {
		fmt.Println(err)
		return
	}
	prev, err := getPrev(c)
	if err != nil {
		fmt.Println(err)
		return
	}
	html, err := getHtmlFor(prev)
	if err != nil {
		fmt.Println(err)
		return
	}

	imagesNext, err := appendImagesToBuf(html, s.ImageBuffers)
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

func (s *Server) handlePrev(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received Prev")
	if s.NextViewModel != nil {
		go func(viewModel ImageViewModel, s *Server) {
			for _, img := range viewModel.Images {
				delete(s.ImageBuffers, img.Path)
			}
			fmt.Println("Cleaned out of scope Last")
		}(*s.NextViewModel, s)
	}

	s.NextViewModel = s.CurrViewModel
	s.CurrViewModel = s.PrevViewModel
	s.NextSubUrl = s.CurrSubUrl
	s.CurrSubUrl = s.PrevSubUrl

	<-s.PrevReady

	go s.loadPrev()

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleCurrent(w http.ResponseWriter, _ *http.Request) {
	tmpl := template.Must(template.ParseFiles("test.gohtml"))
	err := tmpl.Execute(w, s.CurrViewModel)
	if err != nil {
		fmt.Println(err)
	}
}

func (s *Server) HandleNew(w http.ResponseWriter, r *http.Request) {
	title := r.PathValue("title")
	chapter := r.PathValue("chapter")

	url := fmt.Sprintf("/title/%s/%s", title, chapter)

	s.ImageBuffers = make(map[string]*bytes.Buffer)
	s.CurrSubUrl = url
	s.PrevSubUrl = ""
	s.NextSubUrl = ""
	s.loadCurr()

	go s.loadNext()
	go s.loadPrev()

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) loadCurr() {
	html, err := getHtmlFor(s.CurrSubUrl)
	if err != nil {
		panic(err)
	}

	imagesCurr, err := appendImagesToBuf(html, s.ImageBuffers)

	title, chapter, err := getTitleAndChapter(s.CurrSubUrl)
	if err != nil {
		title = "Unknown"
		chapter = "ch_?"
	}

	full := strings.Replace(title, "-", " ", -1) + " - " + strings.Replace(chapter, "_", " ", -1)

	s.CurrViewModel = &ImageViewModel{Images: imagesCurr, Title: full}
	fmt.Println("Loaded current")
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
