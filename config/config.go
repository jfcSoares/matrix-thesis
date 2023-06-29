package config

import (
	"path/filepath"

	"maunium.net/go/gomuks/matrix/rooms"
	"maunium.net/go/mautrix/id"
)

// Config contains the main config of gomuks.
type Config struct {
	UserID      id.UserID   `yaml:"mxid"`
	DeviceID    id.DeviceID `yaml:"device_id"`
	AccessToken string      `yaml:"access_token"`
	Homeserver  string      `yaml:"homeserver"`

	RoomCacheSize int   `yaml:"room_cache_size"`
	RoomCacheAge  int64 `yaml:"room_cache_age"`

	Dir          string `yaml:"-"`
	DataDir      string `yaml:"data_dir"`
	CacheDir     string `yaml:"cache_dir"`
	HistoryPath  string `yaml:"history_path"`
	RoomListPath string `yaml:"room_list_path"`
	StateDir     string `yaml:"state_dir"`

	/*Preferences UserPreferences        `yaml:"-"`*/
	//AuthCache AuthCache        `yaml:"-"`
	Rooms *rooms.RoomCache `yaml:"-"` //not sure if required, for now

	nosave bool
}

// NewConfig creates a config that loads data from the given directory.
func NewConfig(configDir, dataDir, cacheDir string) *Config {
	return &Config{
		Dir:          configDir,
		DataDir:      dataDir,
		CacheDir:     cacheDir,
		HistoryPath:  filepath.Join(cacheDir, "history.db"),
		RoomListPath: filepath.Join(cacheDir, "rooms.gob.gz"),
		StateDir:     filepath.Join(cacheDir, "state"),

		RoomCacheSize: 32,
		RoomCacheAge:  1 * 60,

		/*NotifySound:           true,
		SendToVerifiedOnly:    false,
		Backspace1RemovesWord: true,
		AlwaysClearScreen:     true,*/
	}
}
