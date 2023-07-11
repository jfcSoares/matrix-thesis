// Based on https://github.com/tulir/gomuks/blob/master/gomuks.go
package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"thesgo/config"
	ifc "thesgo/interfaces"
	"thesgo/matrix"
	"time"
)

type Thesgo struct {
	matrix *matrix.ClientWrapper
	config *config.Config
	stop   chan bool
}

func NewThesgo(configDir, dataDir, cacheDir string) *Thesgo {

	thgo := &Thesgo{
		stop: make(chan bool, 1),
	}

	thgo.config = config.NewConfig(configDir, dataDir, cacheDir)
	//thesgo.ui = uiProvider(thgo)
	thgo.matrix = matrix.NewWrapper(thgo.config)

	thgo.config.LoadAll()
	//thgo.ui.Init()

	//debug.OnRecover = thgo.ui.Finish

	return thgo
}

// Save saves the active session and message history.
func (thgo *Thesgo) Save() {
	thgo.config.SaveAll()
}

// StartAutosave calls Save() every minute until it receives a stop signal
// on the Thesgo.stop channel.
func (thgo *Thesgo) StartAutosave() {
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticker.C:
			if thgo.config.AuthCache.InitialSyncDone {
				thgo.Save()
			}
		case val := <-thgo.stop:
			if val {
				return
			}
		}
	}
}

// Stop stops the Matrix syncer and the autosave goroutine,
// then saves everything and calls os.Exit(0).
func (thgo *Thesgo) Stop(save bool) {
	go thgo.internalStop(save)
}

func (thgo *Thesgo) internalStop(save bool) {
	fmt.Print("Disconnecting from Matrix...")
	thgo.matrix.Stop()
	fmt.Print("Cleaning up UI...")
	//thgo.ui.Stop()
	thgo.stop <- true
	if save {
		thgo.Save()
	}
	fmt.Print("Exiting process")
	os.Exit(0)
}

// Start opens a goroutine for the autosave loop and starts the tview app.
//
// If the tview app returns an error, it will be passed into panic(), which
// will be recovered as specified in Recover().
func (thgo *Thesgo) Start() {
	err := thgo.matrix.InitClient(true)
	if err != nil {
		if errors.Is(err, matrix.ErrServerOutdated) {
			_, _ = fmt.Fprintln(os.Stderr, strings.Replace(err.Error(), "homeserver", thgo.config.Homeserver, 1))
			_, _ = fmt.Fprintln(os.Stderr)
			_, _ = fmt.Fprintf(os.Stderr, "See `%s --help` if you want to skip this check or clear all data.\n", os.Args[0])
			os.Exit(4)
		} else if strings.HasPrefix(err.Error(), "failed to check server versions") {
			_, _ = fmt.Fprintln(os.Stderr, "Failed to check versions supported by server:", errors.Unwrap(err))
			_, _ = fmt.Fprintln(os.Stderr)
			_, _ = fmt.Fprintf(os.Stderr, "Modify %s if the server has moved.\n", filepath.Join(thgo.config.Dir, "config.yaml"))
			_, _ = fmt.Fprintf(os.Stderr, "See `%s --help` if you want to skip this check or clear all data.\n", os.Args[0])
			os.Exit(5)
		} else {
			panic(err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		thgo.Stop(true)
	}()

	go thgo.StartAutosave()
	/*if err = thgo.ui.Start(); err != nil {
		panic(err)
	}*/
}

// Matrix returns the MatrixContainer instance.
func (thgo *Thesgo) Matrix() ifc.MatrixContainer {
	return thgo.matrix
}

// Config returns the Thesgo config instance.
func (thgo *Thesgo) Config() *config.Config {
	return thgo.config
}

/*// UI returns the Thesgo UI instance.
func (thgo *Thesgo) UI() ifc.ThesgoUI {
	return thgo.ui
}*/
