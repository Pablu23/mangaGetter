package provider

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

type Asura struct{}

func (a Asura) GetImageList(html string) (imageUrls []string, err error) {
	reg, err := regexp.Compile(`<img decoding="async" class="ts-main-image " src="(.*?)"`)
	if err != nil {
		return nil, err
	}
	m := reg.FindAllStringSubmatch(html, -1)
	l := len(m)
	result := make([]string, l)
	for i, match := range m {
		result[i] = match[1]
	}

	return result, nil
}

func (a Asura) GetHtml(url string) (html string, err error) {
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

func (a Asura) GetNext(html string) (url string, err error) {
	//TODO implement me
	return "#/next/", nil
}

func (a Asura) GetPrev(html string) (url string, err error) {
	//TODO implement me
	return "#/prev/", nil
}

func (a Asura) GetTitleAndChapter(url string) (title string, chapter string, err error) {
	//TODO implement me
	reg, err := regexp.Compile(`\d*-(.*?)-(\d*)/`)
	if err != nil {
		return "", "", err
	}

	matches := reg.FindAllStringSubmatch(url, -1)
	if len(matches) <= 0 {
		return "", "", errors.New("no title or chapter found")
	}

	return matches[0][1], matches[0][2], nil
}

func (a Asura) GetTitleIdAndChapterId(url string) (titleId int, chapterId int, err error) {
	//TODO implement me
	reg, err := regexp.Compile(`(\d*)-.*?-(\d*)/`)
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

	return t, c, nil
}

func (a Asura) GetThumbnail(mangaId string) (thumbnailUrl string, err error) {
	//TODO implement me
	panic("implement me")
}
