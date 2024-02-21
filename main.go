package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
)

type Image struct {
	Path  string
	Index int
}

type ImageViewModel struct {
	Title  string
	Images []Image
}

type MangaViewModel struct {
	Title    string
	Number   int
	LastTime string
	Url      string
}

type MenuViewModel struct {
	Mangas []MangaViewModel
}

func main() {
	db := NewDatabase("db.sqlite", true)
	err := db.Open()
	if err != nil {
		return
	}

	server := Server{
		ImageBuffers: make(map[string]*bytes.Buffer),
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

	http.HandleFunc("/", server.HandleMenu)
	http.HandleFunc("/new/title/{title}/{chapter}", server.HandleNew)
	http.HandleFunc("/current/", server.HandleCurrent)
	http.HandleFunc("/img/{url}/", server.HandleImage)
	http.HandleFunc("POST /next", server.HandleNext)
	http.HandleFunc("POST /prev", server.HandlePrev)
	http.HandleFunc("POST /exit", server.HandleExit)

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
