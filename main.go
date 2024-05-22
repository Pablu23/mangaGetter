package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"time"

	"github.com/pablu23/mangaGetter/internal/database"
	"github.com/pablu23/mangaGetter/internal/provider"
	"github.com/pablu23/mangaGetter/internal/server"
)

func main() {
	openBrowser := true
	var filePath string
	var secret string
	if len(os.Args) >= 2 {
		openBrowser = false
		filePath = os.Args[2]
		buf, err := os.ReadFile(os.Args[3])
		if err != nil {
			panic(err)
		}
		secret = string(buf)
	} else {
		secret = getSecret()
		filePath = getDbPath()
	}

	db := database.NewDatabase(filePath, true)
	err := db.Open()
	if err != nil {
		fmt.Println(err)
		return
	}

  secret = strings.TrimSpace(secret)
	mux := http.NewServeMux()
	s := server.New(&provider.Bato{}, &db, mux, secret)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for range c {
			Close(&db)
		}
	}()

	if openBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			err := open(fmt.Sprintf("http://localhost:%d", port))
			if err != nil {
				fmt.Println(err)
			}
		}()
	}

	err = s.Start(port)
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
	err := db.Close()
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(0)
}
