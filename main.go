package main

import (
	"flag"
	"fmt"
	"io"
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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
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
	debugFlag          = flag.Bool("debug", false, "Activate debug Logs")
	prettyLogsFlag     = flag.Bool("pretty", false, "Pretty pring Logs")
	logPathFlag        = flag.String("log", "", "Path to logfile, stderr if default")
)

func main() {
	var secret string = ""
	var filePath string

	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *prettyLogsFlag {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	if !*debugFlag {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if *logPathFlag != "" {
		var console io.Writer = os.Stderr
		if *prettyLogsFlag {
			console = zerolog.ConsoleWriter{Out: os.Stderr}
		}
		log.Logger = log.Output(zerolog.MultiLevelWriter(console, &lumberjack.Logger{
			Filename:   *logPathFlag,
			MaxAge:     14,
			MaxBackups: 10,
		}))
	}

	if *secretFlag != "" {
		secret = *secretFlag
	} else if *secretFilePathFlag != "" {
		buf, err := os.ReadFile(*secretFilePathFlag)
		if err != nil {
			log.Fatal().Err(err).Str("Path", *secretFilePathFlag).Msg("Could not read secret File")
		}
		secret = string(buf)
	} else if *authFlag {
		cacheSecret, err := getSecret()
		secret = cacheSecret
		if err != nil {
			log.Error().Err(err).Msg("Secret file could not be found or read, not activating Auth")
		}
	}

	if *databaseFlag != "" {
		filePath = *databaseFlag
	} else {
		filePath = getDbPath()
	}

	db := database.NewDatabase(filePath, true, *debugFlag)
	err := db.Open()
	if err != nil {
		log.Fatal().Err(err).Str("Path", filePath).Msg("Could not open Database")
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
				log.Error().Err(err).Msg("Could not open Browser")
			}
		}()
	}

	interval, err := time.ParseDuration(*updateIntervalFlag)
	if err != nil {
		log.Fatal().Err(err).Str("Interval", *updateIntervalFlag).Msg("Could not parse interval")
	}
	s.RegisterUpdater(interval)
	s.RegisterRoutes()

	if *certFlag != "" && *keyFlag != "" {
		err = s.StartTLS(*portFlag, *certFlag, *keyFlag)
		if err != nil {
			log.Fatal().Err(err).Str("Cert", *certFlag).Str("Key", *keyFlag).Int("Port", *portFlag).Msg("Could not start TLS server")
		}
	} else {
		err = s.Start(*portFlag)
		if err != nil {
			log.Fatal().Err(err).Int("Port", *portFlag).Msg("Could not start server")
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
	log.Debug().Msg("Closing Database")
	err := db.Close()
	if err != nil {
		log.Error().Err(err).Msg("Could not close Database")
		return
	}
	os.Exit(0)
}
