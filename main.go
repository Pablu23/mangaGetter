package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

type Image struct {
	Path  string
	Index int
}

type ImageData struct {
	Images []Image
}

func main() {
	resp, err := http.Get("https://bato.to/title/80381-i-stan-the-prince/1525068-ch_1?load=2")
	// TODO: Testing for above 300 is dirty
	if err != nil && resp.StatusCode > 300 {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Could not close body because: %v\n", err)
		}
	}(resp.Body)

	create, err := os.Create("h.html")
	if err != nil {
		panic(err)
	}
	defer func(create *os.File) {
		err := create.Close()
		if err != nil {
			fmt.Printf("Could not close file because: %v\n", err)
		}
	}(create)

	all, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	_, err = create.Write(all)
	if err != nil {
		panic(err)
	}

	reg, err := regexp.Compile(`<astro-island.*props=".*;imageFiles&quot;:\[1,&quot;\[(.*)]&quot;]`)
	if err != nil {
		panic(err)
	}

	h := string(all)
	match := reg.FindStringSubmatch(h)[1]
	reg, err = regexp.Compile(`\[0,\\&quot;([^&]*)\\&quot;]`)
	if err != nil {
		panic(err)
	}
	findings, err := os.Create("findings.txt")
	if err != nil {
		panic(err)
	}
	defer func(findings *os.File) {
		err := findings.Close()
		if err != nil {
			fmt.Printf("Could not close file because: %v\n", err)
		}
	}(findings)

	matches := reg.FindAllStringSubmatch(match, -1)
	images := make([]Image, 0)

	imageBufs := make(map[string]*bytes.Buffer)

	for i, m := range matches {
		//err = downloadFile(m[1])
		buf, err := addFileToRam(m[1])
		if err != nil {
			panic(err)
		}
		name := filepath.Base(m[1])
		imageBufs[name] = buf
		images = append(images, Image{Path: name, Index: i})
	}

	tmpl := template.Must(template.ParseFiles("test.html"))

	data := ImageData{Images: images}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, data)
	})
	http.HandleFunc("/{url}/", func(w http.ResponseWriter, r *http.Request) {
		u := r.PathValue("url")
		buf := imageBufs[u]

		w.Header().Set("Content-Type", "image/webp")
		buf.WriteTo(w)
	})
	fmt.Println("Server running")
	http.ListenAndServe(":8000", nil)
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

func downloadFile(url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath.Base(url))
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
