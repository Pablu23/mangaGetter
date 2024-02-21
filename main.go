package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
)

type NoMoreError struct {
	Err error
}

func (e *NoMoreError) Error() string {
	return e.Err.Error()
}

type Image struct {
	Path  string
	Index int
}

type ImageViewModel struct {
	Title  string
	Images []Image
}

func main() {
	db := NewDatabase("db.sqlite", true)
	err := db.Open()
	if err != nil {
		return
	}

	var latestTimeStamp int64 = 0
	var latestUrl string
	for _, m := range db.Mangas {
		if latestTimeStamp < m.LatestChapter.TimeStampUnix {
			latestTimeStamp = m.LatestChapter.TimeStampUnix
			latestUrl = m.LatestChapter.Url
		}
	}

	if latestUrl == "" {
		latestUrl = "/title/80381-i-stan-the-prince/1539086-ch_16"
	}

	server := Server{
		ImageBuffers: make(map[string]*bytes.Buffer),
		CurrSubUrl:   latestUrl,
		NextReady:    make(chan bool),
		PrevReady:    make(chan bool),
		Provider:     &Bato{},
		DbMgr:        &db,
		Mutex:        &sync.Mutex{},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for range c {
			Close(&db)
		}
	}()

	server.LoadCurr()
	go server.LoadPrev()
	go server.LoadNext()

	http.HandleFunc("/", server.HandleCurrent)
	http.HandleFunc("/img/{url}/", server.HandleImage)
	http.HandleFunc("POST /next", server.HandleNext)
	http.HandleFunc("POST /prev", server.HandlePrev)
	http.HandleFunc("POST /exit", func(_ http.ResponseWriter, _ *http.Request) {
		Close(&db)
	})
	http.HandleFunc("/new/{title}/{chapter}", server.HandleNew)

	fmt.Println("Server starting...")
	err = http.ListenAndServe(":8000", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func Close(db *DatabaseManager) {
	fmt.Println("Attempting to save and close DB")
	err := db.Save()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = db.Close()
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(0)
}
