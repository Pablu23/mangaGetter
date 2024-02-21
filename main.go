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
	"sync"
)

type Image struct {
	Path  string
	Index int
}

type ImageViewModel struct {
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
}

func getImageList(html string) ([]string, error) {
	reg, err := regexp.Compile(`<astro-island.*props=".*;imageFiles&quot;:\[1,&quot;\[(.*)]&quot;]`)
	if err != nil {
		return nil, err
	}
	m := reg.FindStringSubmatch(html)

	if len(m) <= 0 {
		return nil, errors.New("no new images")
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
	h, err := getHtmlFor(curr)
	if err != nil {
		panic(err)
	}

	n, err := getNext(h)
	if err != nil {
		panic(err)
	}

	p, err := getPrev(h)
	if err != nil {
		panic(err)
	}

	server := Server{
		PrevViewModel: nil,
		CurrViewModel: nil,
		NextViewModel: nil,
		ImageBuffers:  make(map[string]*bytes.Buffer),
		// Weird error IDK why this is, but it works like this, so it is what it is
		NextSubUrl: n, //"/title/143267-blooming-love/2636103-ch_20",
		PrevSubUrl: p,
		CurrSubUrl: curr,
	}

	go func(s *Server) {
		html, err := getHtmlFor(s.PrevSubUrl)
		if err != nil {
			fmt.Println(err)
			return
		}

		imagesNext, err := appendImagesToBuf(html, s.ImageBuffers)
		if err != nil {
			fmt.Println(err)
			return
		}

		s.PrevViewModel = &ImageViewModel{Images: imagesNext}
		fmt.Println("Finished loading prev")
	}(&server)

	go func(s *Server) {
		html, err := getHtmlFor(s.NextSubUrl)
		if err != nil {
			fmt.Println(err)
			return
		}

		imagesNext, err := appendImagesToBuf(html, s.ImageBuffers)
		if err != nil {
			fmt.Println(err)
			return
		}
		s.NextViewModel = &ImageViewModel{Images: imagesNext}
		fmt.Println("Finished loading next")
	}(&server)

	imagesCurr, err := appendImagesToBuf(h, server.ImageBuffers)

	server.CurrViewModel = &ImageViewModel{Images: imagesCurr}

	http.HandleFunc("/", server.HandleCurrent)
	http.HandleFunc("/img/{url}/", server.HandleImage)
	http.HandleFunc("POST /next", server.handleNext)
	http.HandleFunc("POST /prev", server.handlePrev)

	fmt.Println("Server running")
	err = http.ListenAndServe(":8000", nil)
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

	go func(s *Server) {
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
		if err != nil {
			fmt.Println(err)
			return
		}

		s.NextViewModel = &ImageViewModel{Images: imagesNext}

		s.NextSubUrl = next
		fmt.Println("Loaded next")
	}(s)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
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

	go func(s *Server) {
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

		s.PrevViewModel = &ImageViewModel{Images: imagesNext}

		s.PrevSubUrl = prev
		fmt.Println("Loaded prev")
	}(s)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleCurrent(w http.ResponseWriter, _ *http.Request) {
	tmpl := template.Must(template.ParseFiles("test.html"))
	err := tmpl.Execute(w, s.CurrViewModel)
	if err != nil {
		fmt.Println(err)
	}
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
