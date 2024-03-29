package main

import (
	"fmt"
	"mangaGetter/internal/database"
	"mangaGetter/internal/provider"
	"mangaGetter/internal/server"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"
)

func main() {
	filePath := getDbPath()

	db := database.NewDatabase(filePath, true)
	err := db.Open()
	if err != nil {
		fmt.Println(err)
		return
	}

	s := server.New(&provider.Bato{}, &db)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for range c {
			Close(&db)
		}
	}()

	go func() {
		time.Sleep(300 * time.Millisecond)
		err := open("http://localhost:8000")
		if err != nil {
			fmt.Println(err)
		}
	}()

	err = s.Start()
	if err != nil {
		panic(err)
	}
}

func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func Close(db *database.Manager) {
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
