/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"thesgo/cmd"
	deb "thesgo/debug" //matrix client logger

	debug "maunium.net/go/gomuks/debug" //general application logger
)

func main() {

	debugDir := os.Getenv("DEBUG_DIR")
	if len(debugDir) > 0 {
		debug.LogDirectory = debugDir
		deb.LogDirectory = debugDir
	}

	debug.DeadlockDetection = true
	debug.WriteLogs = true
	debug.RecoverPrettyPanic = false
	debug.Initialize() //Initialize general app logger
	defer debug.Recover()

	var configDir, dataDir, cacheDir string
	var err error

	configDir, err = UserConfigDir()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Failed to get config directory:", err)
		os.Exit(3)
	}
	dataDir, err = UserDataDir()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Failed to get data directory:", err)
		os.Exit(3)
	}
	cacheDir, err = UserCacheDir()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "Failed to get cache directory:", err)
		os.Exit(3)
	}

	debug.Print("Config directory:", configDir)
	debug.Print("Data directory:", dataDir)
	debug.Print("Cache directory:", cacheDir)

	thesgo := NewThesgo(configDir, dataDir, cacheDir)
	thesgo.Start()
	cmd.SetLinkToBackend(thesgo) //link cli to rest of the client code
	defer cmd.Execute()          //run the interface after initial setup has finished

}

func getRootDir(subdir string) string {
	rootDir := os.Getenv("THESGO_ROOT")
	if rootDir == "" { //if env variable isnt set, return empty
		return ""
	}
	return filepath.Join(rootDir, subdir) //else return the path to the new subdirectory
}

func UserCacheDir() (dir string, err error) {
	dir = os.Getenv("THESGO_CACHE_HOME")
	if dir == "" { //if env variable isnt set, try to create subdirectory under root
		dir = getRootDir("cache")
	}
	if dir == "" { //if that fails, get default OS directory and create a folder for the program there
		dir, err = os.UserCacheDir()
		dir = filepath.Join(dir, "thesgo")
	}
	return
}

func UserDataDir() (dir string, err error) {
	dir = os.Getenv("THESGO_DATA_HOME")
	if dir != "" {
		return
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return UserConfigDir()
	}
	dir = getRootDir("data")
	if dir == "" {
		dir = os.Getenv("XDG_DATA_HOME")
	}
	if dir == "" {
		dir = os.Getenv("HOME")
		if dir == "" {
			return "", errors.New("neither $XDG_DATA_HOME nor $HOME are defined")
		}
		dir = filepath.Join(dir, ".local", "share")
	}
	dir = filepath.Join(dir, "thesgo")
	return
}

func UserConfigDir() (dir string, err error) {
	dir = os.Getenv("THESGO_CONFIG_HOME")
	if dir == "" {
		dir = getRootDir("config")
	}
	if dir == "" {
		dir, err = os.UserConfigDir()
		dir = filepath.Join(dir, "thesgo")
	}
	return
}
