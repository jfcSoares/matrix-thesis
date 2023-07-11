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
	"thesgo/matrix/mxevents"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func main() {

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

	fmt.Println("Config directory:", configDir)
	fmt.Println("Data directory:", dataDir)
	fmt.Println("Cache directory:", cacheDir)

	thesgo := NewThesgo(configDir, dataDir, cacheDir)
	cmd.SetLinkToBackend(thesgo) //link interface to rest of the client code
	cmd.Execute()                //run the interface

	thesgo.Start()
	c := thesgo.Matrix()
	c.Login("test1", "Test1!´´´")

	rooms, _ := c.RoomsJoined()
	fmt.Println(event.StateEncryption)

	stateKey := ""
	evt := mxevents.Wrap(&event.Event{
		Type:     event.StateEncryption,
		RoomID:   rooms[0],
		StateKey: &stateKey,
		Content: event.Content{Parsed: &event.EncryptionEventContent{
			Algorithm:              id.AlgorithmMegolmV1,
			RotationPeriodMillis:   604800000, //for now use default session rotation
			RotationPeriodMessages: 100,
		}},
	})

	c.SendStateEvent(evt)

	/*c := matrix.NewWrapper()
	c.InitClient(false)

	/*content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    "Hello World!",
	}

	evt := mxevents.Wrap(&event.Event{
		ID:       id.EventID(c.Client().TxnID()),
		Sender:   c.Client().UserID,
		Type:     event.EventMessage,
		RoomID:   rooms[0],
		Content:  event.Content{Parsed: content},
		Unsigned: event.Unsigned{TransactionID: c.Client().TxnID()},
	})

	c.SendEvent(evt)*/
	//c.Start()

	//<-c.IsStopped()
	c.Logout()
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
	dir = os.Getenv("GOMUKS_CONFIG_HOME")
	if dir == "" {
		dir = getRootDir("config")
	}
	if dir == "" {
		dir, err = os.UserConfigDir()
		dir = filepath.Join(dir, "gomuks")
	}
	return
}
