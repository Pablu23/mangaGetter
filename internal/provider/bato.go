package provider

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

type Bato struct{}

func (b *Bato) GetImageList(html string) ([]string, error) {
	reg, err := regexp.Compile(`<astro-island.*props=".*;imageFiles&quot;:\[1,&quot;\[(.*)]&quot;]`)
	if err != nil {
		return nil, err
	}
	m := reg.FindStringSubmatch(html)

	if len(m) <= 0 {
		return nil, errors.New("no more content")
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

func (b *Bato) GetTitleAndChapter(url string) (title string, chapter string, err error) {
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

func (b *Bato) GetTitleIdAndChapterId(url string) (titleId int, chapterId int, err error) {
	reg, err := regexp.Compile(`/title/(\d*)-.*?/(\d*)-.*`)
	if err != nil {
		return 0, 0, err
	}

	matches := reg.FindAllStringSubmatch(url, -1)
	if len(matches) <= 0 {
		return 0, 0, errors.New("no title or chapter found")
	}
	t, err := strconv.Atoi(matches[0][1])
	if err != nil {
		return 0, 0, err
	}
	c, err := strconv.Atoi(matches[0][2])

	return t, c, err
}

func (b *Bato) GetChapterList(subUrl string) (subUrls []string, err error) {
	reg, err := regexp.Compile(`<div class="space-x-1">.*?<a href="(.*?)" .*?>.*?</a>`)
	if err != nil {
		return nil, err
	}

	html, err := b.GetHtml(subUrl)
	if err != nil {
		return nil, err
	}

	subUrls = make([]string, 0)
	matches := reg.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		subUrls = append(subUrls, match[1])
	}
	return subUrls, nil
}

func (b *Bato) GetThumbnail(subUrl string) (thumbnailUrl string, err error) {
	url := fmt.Sprintf("https://bato.to/title/%s", subUrl)
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

	reg, err := regexp.Compile(`<img data-hk="0-1-0" .*? src="(.*?)["']`)
	if err != nil {
		return "", err
	}
	match := reg.FindStringSubmatch(h)
	if len(match) <= 1 {
		return "", errors.New("could not find Thumbnail url")
	}

	return match[1], nil
}
