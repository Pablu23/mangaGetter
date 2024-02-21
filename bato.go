package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

type Provider interface {
	GetImageList(html string) (imageUrls []string, err error)
	GetHtml(url string) (html string, err error)
	GetNext(html string) (url string, err error)
	GetPrev(html string) (url string, err error)
}

type Bato struct{}

func (b *Bato) GetImageList(html string) ([]string, error) {
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

func (b *Bato) GetHtml(titleSubUrl string) (string, error) {
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

func (b *Bato) GetNext(html string) (subUrl string, err error) {
	reg, err := regexp.Compile(`<a data-hk="0-6-0" .*? href="(.*?)["']`)
	match := reg.FindStringSubmatch(html)

	return match[1], err
}

func (b *Bato) GetPrev(html string) (subUrl string, err error) {
	reg, err := regexp.Compile(`<a data-hk="0-5-0" .*? href="(.*?)["']`)
	match := reg.FindStringSubmatch(html)

	return match[1], err
}
