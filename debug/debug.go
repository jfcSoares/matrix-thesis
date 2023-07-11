package debug

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var LogDirectory = GetUserDebugDir()

func GetUserDebugDir() string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return filepath.Join(os.TempDir(), "thesgo-"+getUname())
	}
	// See https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
	if xdgStateHome := os.Getenv("XDG_STATE_HOME"); xdgStateHome != "" {
		return filepath.Join(xdgStateHome, "thesgo")
	}
	home := os.Getenv("HOME")
	if home == "" {
		fmt.Println("XDG_STATE_HOME and HOME are both unset")
		os.Exit(1)
	}
	return filepath.Join(home, ".local", "state", "thesgo")
}

func getUname() string {
	currUser, err := user.Current()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return currUser.Username
}

func Initialize() *zerolog.Logger {
	err := os.MkdirAll(LogDirectory, 0750)
	if err != nil {
		return nil
	}

	file, err := os.OpenFile(filepath.Join(LogDirectory, "debug.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		panic(err)
	}

	defer file.Close() //TODO: this closes file right after initializing the logger, which defeats the purpose
	//Change this flow in order to keep file open at all times (possibly only opening in main.main())

	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        file,
		TimeFormat: time.Now().Format("02-01-2006 15:04:05"),
		FormatLevel: func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("[%s]", i))
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("| %s |", i)
		},
		FormatCaller: func(i interface{}) string {
			return filepath.Base(fmt.Sprintf("%s", i))
		},
		PartsExclude: []string{
			zerolog.TimestampFieldName,
		},
	}).With().Timestamp().Caller().Logger()
	logger.Level(zerolog.DebugLevel)

	logger.Info().Msg("======================= Debug init @ " + time.Now().Format("02-01-2006 15:04:05") + " =======================\n")

	return &logger
}
