package main

import (
	"flag"
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

var (
	secretFlag         = flag.String("secret", "", "Secret to use for Auth")
	authFlag           = flag.Bool("auth", false, "Use Auth, does not need to be set if secret or secret-path is set")
	secretFilePathFlag = flag.String("secret-path", "", "Path to file with ONLY secret in it")
	portFlag           = flag.Int("port", 80, "The port on which to host")
	serverFlag         = flag.Bool("server", false, "If false dont open Browser with Address")
	databaseFlag       = flag.String("database", "", "Path to sqlite.db file")
	certFlag           = flag.String("cert", "", "Path to cert file, has to be used in conjunction with key")
	keyFlag            = flag.String("key", "", "Path to key file, has to be used in conjunction with cert")
	updateIntervalFlag = flag.String("update", "1h", "Interval to update Mangas")
)

func main() {
	var secret string = ""
	var filePath string

	flag.Parse()
	if *secretFlag != "" {
		secret = *secretFlag
	} else if *secretFilePathFlag != "" {
		buf, err := os.ReadFile(*secretFilePathFlag)
		if err != nil {
			panic(err)
		}
		secret = string(buf)
	} else if *authFlag {
		cacheSecret, err := getSecret()
		secret = cacheSecret
		if err != nil {
			fmt.Printf("Secret file could not be found or read because of %s, not activating Auth\n", err)
		}
	}

	if *databaseFlag != "" {
		filePath = *databaseFlag
	} else {
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

	if !*serverFlag {
		go func() {
			time.Sleep(300 * time.Millisecond)
			err := open(fmt.Sprintf("http://localhost:%d", *portFlag))
			if err != nil {
				fmt.Println(err)
			}
		}()
	}

	interval, err := time.ParseDuration(*updateIntervalFlag)
	if err != nil {
		panic(err)
	}
	s.RegisterUpdater(interval)
	s.RegisterRoutes()

	if *certFlag != "" && *keyFlag != "" {
		err = s.StartTLS(*portFlag, *certFlag, *keyFlag)
		if err != nil {
			panic(err)
		}
	} else {
		err = s.Start(*portFlag)
		if err != nil {
			panic(err)
		}
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
