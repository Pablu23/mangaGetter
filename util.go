package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

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

func getMangaIdAndChapterId(url string) (titleId int, chapterId int, err error) {
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
