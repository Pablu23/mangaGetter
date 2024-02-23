package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"mangaGetter/internal/database"
	"mangaGetter/internal/provider"
	"mangaGetter/internal/server"
)

func main() {
	db := database.NewDatabase("db.sqlite", true)
	err := db.Open()
	if err != nil {
		return
	}

	s := server.Server{
		ImageBuffers: make(map[string]*bytes.Buffer),
		NextReady:    make(chan bool),
		PrevReady:    make(chan bool),
		Provider:     &provider.Bato{},
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

	http.HandleFunc("/", s.HandleMenu)
	http.HandleFunc("/new/title/{title}/{chapter}", s.HandleNew)
	http.HandleFunc("/current/", s.HandleCurrent)
	http.HandleFunc("/img/{url}/", s.HandleImage)
	http.HandleFunc("POST /next", s.HandleNext)
	http.HandleFunc("POST /prev", s.HandlePrev)
	http.HandleFunc("POST /exit", s.HandleExit)

	fmt.Println("Server starting...")
	err = http.ListenAndServe(":8000", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func Close(db *database.DatabaseManager) {
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
