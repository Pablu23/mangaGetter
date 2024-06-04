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
	updateIntervalFlag = flag.String("update", "0h", "Interval to update Mangas")
	debugFlag          = flag.Bool("debug", false, "Activate debug Logs")
	prettyLogsFlag     = flag.Bool("pretty", false, "Pretty pring Logs")
	logPathFlag        = flag.String("log", "", "Path to logfile, stderr if default")
)

func main() {
	flag.Parse()

	setupLogging()

	filePath := setupDb()
	db := database.NewDatabase(filePath, true, *debugFlag)
	err := db.Open()
	if err != nil {
		log.Fatal().Err(err).Str("Path", filePath).Msg("Could not open Database")
	}

	mux := http.NewServeMux()
	s := server.New(&provider.Bato{}, &db, mux, func(o *server.Options) {
		authOptions := setupAuth()
		o.Auth.Set(authOptions)
		interval, err := time.ParseDuration(*updateIntervalFlag)
		if err != nil {
			log.Fatal().Err(err).Str("Interval", *updateIntervalFlag).Msg("Could not parse interval")
		}
		o.UpdateInterval = interval

		if *certFlag != "" && *keyFlag != "" {
			o.Tls.Apply(func(to *server.TlsOptions) {
				to.CertPath = *certFlag
				to.KeyPath = *keyFlag
			})
		}
	})

	setupClient()
	setupClose(&db)
	err = s.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("Could not start server")
	}
}

func setupAuth() server.AuthOptions {
	var authOptions server.AuthOptions
	if *secretFlag != "" {
		authOptions.LoadType = server.Raw
		authOptions.Secret = *secretFlag
	} else if *secretFilePathFlag != "" {
		authOptions.LoadType = server.File
		authOptions.Secret = *secretFilePathFlag
	} else if *authFlag {
		path, err := getSecretPath()
		if err != nil {
			log.Fatal().Err(err).Msg("Secret file could not be found")
		}
		authOptions.Secret = path
		authOptions.LoadType = server.File
	}
	return authOptions
}

func setupClient() {
	if !*serverFlag {
		go func() {
			time.Sleep(300 * time.Millisecond)
			err := open(fmt.Sprintf("http://localhost:%d", *portFlag))
			if err != nil {
				log.Error().Err(err).Msg("Could not open Browser")
			}
		}()
	}
}

func setupClose(db *database.Manager) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		for range c {
			Close(db)
		}
	}()
}

func setupDb() string {
	if *databaseFlag != "" {
		return *databaseFlag
	} else {
		return getDbPath()
	}
}

func setupLogging() {
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
