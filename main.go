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
	reg, err := regexp.Compile(`<a data-hk="0-6-0" .* href="(.*?)["']`)
	match := reg.FindStringSubmatch(html)

	return match[1], err
}

func appendImagesToBuf(lastHtml string, imageBufs map[string]*bytes.Buffer) ([]Image, error) {
	next, err := getNext(lastHtml)
	if err != nil {
		return nil, err
	}

	html, err := getHtmlFor(next)
	if err != nil {
		return nil, err
	}

	imgList, err := getImageList(html)
	if err != nil {
		return nil, err
	}

	images := make([]Image, 0)

	for i, url := range imgList {
		buf, err := addFileToRam(url)
		if err != nil {
			panic(err)
		}
		name := filepath.Base(url)
		imageBufs[name] = buf
		images = append(images, Image{Path: name, Index: i})
	}

	return images, nil
}

func main() {
	h, err := getHtmlFor("/title/143267-blooming-love/2636103-ch_20")
	if err != nil {
		panic(err)
	}

	imgList, err := getImageList(h)
	if err != nil {
		panic(err)
	}

	server := Server{
		PrevViewModel: nil,
		CurrViewModel: nil,
		NextViewModel: nil,
		ImageBuffers:  make(map[string]*bytes.Buffer),
		// Weird error IDK why this is, but it works like this, so it is what it is
		NextSubUrl: "/title/143267-blooming-love/2636103-ch_20",
	}

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
		next, err := getNext(html)
		if err != nil {
			fmt.Println(err)
			return
		}

		s.NextSubUrl = next
		fmt.Println("Finished loading next")
	}(&server)

	images := make([]Image, 0)
	for i, url := range imgList {
		buf, err := addFileToRam(url)
		if err != nil {
			panic(err)
		}
		name := filepath.Base(url)
		server.ImageBuffers[name] = buf
		images = append(images, Image{Path: name, Index: i})
	}

	server.CurrViewModel = &ImageViewModel{Images: images}

	http.HandleFunc("/", server.HandleCurrent)
	http.HandleFunc("/{url}/", server.HandleImage)
	http.HandleFunc("POST /next", server.handleNext)

	fmt.Println("Server running")
	http.ListenAndServe(":8000", nil)
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
	w.Write(buf.Bytes())
}

func (s *Server) handleNext(w http.ResponseWriter, r *http.Request) {
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
		next, err := getNext(html)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(next)

		s.NextSubUrl = next
		fmt.Println("Loaded next")
	}(s)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleCurrent(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("test.html"))
	tmpl.Execute(w, s.CurrViewModel)
}

func addFileToRam(url string) (*bytes.Buffer, error) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)

	// Write the body to file
	_, err = io.Copy(buf, resp.Body)
	return buf, err
}
