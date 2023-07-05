package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	/*matrix "thesgo/matrix"
	"thesgo/matrix/mxevents"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"*/)

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

	fmt.Print("Config directory:", configDir)
	fmt.Print("Data directory:", dataDir)
	fmt.Print("Cache directory:", cacheDir)

	thgo := NewThesgo(configDir, dataDir, cacheDir)
	fmt.Println(thgo)

	//Provavelmente falta codigo para lidar com flags

	thgo.Start()
	c := thgo.Matrix()
	c.Login("test1", "Test1!´´´")

	/*c := matrix.NewWrapper()
	c.InitClient(false)
	//roomID, _ := c.NewRoom("Test Room", "For testing", nil)

	rooms, _ := c.RoomsJoined()
	c.JoinedMembers(rooms[0])

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
	c.Start()

	//<-c.IsStopped()
	c.Logout()
}

func getRootDir(subdir string) string {
	rootDir := os.Getenv("THESGO_ROOT")
	fmt.Println(rootDir)
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
