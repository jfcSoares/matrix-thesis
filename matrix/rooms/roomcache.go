// Based on https://github.com/tulir/gomuks/blob/master/matrix/rooms/roomcache.go

package rooms

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sync "github.com/sasha-s/go-deadlock"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// RoomCache contains room state info in a hashmap and linked list => essentially is the state store of the client
type RoomCache struct {
	sync.Mutex

	listPath  string
	directory string
	maxSize   int
	maxAge    int64
	getOwner  func() id.UserID
	noUnload  bool

	Map  map[id.RoomID]*Room
	head *Room
	tail *Room
	size int
}

func NewRoomCache(listPath, directory string, maxSize int, maxAge int64, getOwner func() id.UserID) *RoomCache {
	return &RoomCache{
		listPath:  listPath,
		directory: directory,
		maxSize:   maxSize,
		maxAge:    maxAge,
		getOwner:  getOwner,

		Map: make(map[id.RoomID]*Room),
	}
}

func (cache *RoomCache) DisableUnloading() {
	cache.noUnload = true
}

func (cache *RoomCache) EnableUnloading() {
	cache.noUnload = false
}

func (cache *RoomCache) IsEncrypted(roomID id.RoomID) bool {
	room := cache.Get(roomID)
	return room != nil && room.Encrypted
}

func (cache *RoomCache) GetEncryptionEvent(roomID id.RoomID) *event.EncryptionEventContent {
	room := cache.Get(roomID)
	evt := room.GetStateEvent(event.StateEncryption, "")
	if evt == nil {
		return nil
	}
	content, ok := evt.Content.Parsed.(*event.EncryptionEventContent)
	if !ok {
		return nil
	}
	return content
}

func (cache *RoomCache) FindSharedRooms(userID id.UserID) (shared []id.RoomID) {
	// FIXME this disables unloading so TouchNode wouldn't try to double-lock
	cache.DisableUnloading()
	cache.Lock()
	for _, room := range cache.Map {
		if !room.Encrypted {
			continue
		}
		member, ok := room.GetMembers()[userID]
		if ok && member.Membership == event.MembershipJoin {
			shared = append(shared, room.ID)
		}
	}
	cache.Unlock()
	cache.EnableUnloading()
	return
}

// Fetches the room list from the local db file
func (cache *RoomCache) LoadList() error {
	cache.Lock()
	defer cache.Unlock()

	// Open room list file
	file, err := os.OpenFile(cache.listPath, os.O_RDONLY, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open room list file for reading: %w", err)
	}
	defer fmtPrintError(file.Close, "Failed to close room list file after reading")

	// Open gzip reader for room list file
	cmpReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to read gzip room list: %w", err)
	}
	defer fmtPrintError(cmpReader.Close, "Failed to close room list gzip reader")

	// Open gob decoder for gzip reader
	dec := gob.NewDecoder(cmpReader)
	// Read number of items in list
	var size int
	err = dec.Decode(&size)
	if err != nil {
		return fmt.Errorf("failed to read size of room list: %w", err)
	}

	// Read list
	cache.Map = make(map[id.RoomID]*Room, size)
	for i := 0; i < size; i++ {
		room := &Room{}
		err = dec.Decode(room)
		if err != nil {
			fmt.Printf("Failed to decode %dth room list entry: %v", i+1, err)
			continue
		}
		room.path = cache.roomPath(room.ID)
		room.cache = cache
		cache.Map[room.ID] = room
	}
	return nil
}

func (cache *RoomCache) SaveLoadedRooms() {
	cache.Lock()
	cache.clean(false)
	for node := cache.head; node != nil; node = node.prev {
		node.Save()
	}
	cache.Unlock()
}

// Saves the local cache to persistent memory
func (cache *RoomCache) SaveList() error {
	cache.Lock()
	defer cache.Unlock()

	fmt.Print("Saving room list...")
	// Open room list file
	file, err := os.OpenFile(cache.listPath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("failed to open room list file for writing: %w", err)
	}
	defer fmtPrintError(file.Close, "Failed to close room list file after writing")

	// Open gzip writer for room list file
	cmpWriter := gzip.NewWriter(file)
	defer fmtPrintError(cmpWriter.Close, "Failed to close room list gzip writer")

	// Open gob encoder for gzip writer
	enc := gob.NewEncoder(cmpWriter)
	// Write number of items in list
	err = enc.Encode(len(cache.Map))
	if err != nil {
		return fmt.Errorf("failed to write size of room list: %w", err)
	}

	// Write list
	for _, node := range cache.Map {
		err = enc.Encode(node)
		if err != nil {
			fmt.Printf("Failed to encode room list entry of %s: %v", node.ID, err)
		}
	}
	fmt.Print("Room list saved to", cache.listPath, len(cache.Map), cache.size)
	return nil
}

func (cache *RoomCache) Touch(roomID id.RoomID) {
	cache.Lock()
	node, ok := cache.Map[roomID]
	if !ok || node == nil {
		cache.Unlock()
		return
	}
	cache.touch(node)
	cache.Unlock()
}

func (cache *RoomCache) TouchNode(node *Room) {
	if cache.noUnload || node.touch+2 > time.Now().Unix() {
		return
	}
	cache.Lock()
	cache.touch(node)
	cache.Unlock()
}

func (cache *RoomCache) touch(node *Room) {
	if node == cache.head {
		return
	}
	fmt.Print("Touching", node.ID)
	cache.llPop(node)
	cache.llPush(node)
	node.touch = time.Now().Unix()
}

func (cache *RoomCache) Get(roomID id.RoomID) *Room {
	cache.Lock()
	node := cache.get(roomID)
	cache.Unlock()
	return node
}

func (cache *RoomCache) GetOrCreate(roomID id.RoomID) *Room {
	cache.Lock()
	node := cache.get(roomID)
	if node == nil {
		node = cache.newRoom(roomID)
		cache.llPush(node)
	}
	cache.Unlock()
	return node
}

// Attempts to obtain a given room from the local cache
func (cache *RoomCache) get(roomID id.RoomID) *Room {
	node, ok := cache.Map[roomID]
	if ok && node != nil {
		return node
	}
	return nil
}

// Updates a given room object in the cache
func (cache *RoomCache) Put(room *Room) {
	cache.Lock()
	node := cache.get(room.ID)
	if node != nil {
		cache.touch(node)
	} else {
		cache.Map[room.ID] = room
		if room.Loaded() {
			cache.llPush(room)
		}
		node = room
	}
	cache.Unlock()
	node.Save()
}

func (cache *RoomCache) roomPath(roomID id.RoomID) string {
	escapedRoomID := strings.ReplaceAll(strings.ReplaceAll(string(roomID), "%", "%25"), "/", "%2F")
	return filepath.Join(cache.directory, escapedRoomID+".gob.gz")
}

// Loads a new room into memory
func (cache *RoomCache) Load(roomID id.RoomID) *Room {
	cache.Lock()
	defer cache.Unlock()
	node, ok := cache.Map[roomID]
	if ok {
		return node
	}

	node = NewRoom(roomID, cache)
	node.Load()
	return node
}

func (cache *RoomCache) llPop(node *Room) {
	if node.prev == nil && node.next == nil {
		return
	}
	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	if node == cache.tail {
		cache.tail = node.next
	}
	if node == cache.head {
		cache.head = node.prev
	}
	node.next = nil
	node.prev = nil
	cache.size--
}

func (cache *RoomCache) llPush(node *Room) {
	if node.next != nil || node.prev != nil {
		//fmt.PrintStack()
		fmt.Print("Tried to llPush node that is already in stack")
		return
	}
	if node == cache.head {
		return
	}
	if cache.head != nil {
		cache.head.next = node
	}
	node.prev = cache.head
	node.next = nil
	cache.head = node
	if cache.tail == nil {
		cache.tail = node
	}
	cache.size++
	cache.clean(false)
}

func (cache *RoomCache) ForceClean() {
	cache.Lock()
	cache.clean(true)
	cache.Unlock()
}

func (cache *RoomCache) clean(force bool) {
	if cache.noUnload && !force {
		return
	}
	origSize := cache.size
	maxTS := time.Now().Unix() - cache.maxAge
	for cache.size > cache.maxSize {
		if cache.tail.touch > maxTS && !force {
			break
		}
		ok := cache.tail.Unload()
		node := cache.tail
		cache.llPop(node)
		if !ok {
			fmt.Print("Unload returned false, pushing node back")
			cache.llPush(node)
		}
	}
	if cleaned := origSize - cache.size; cleaned > 0 {
		fmt.Print("Cleaned", cleaned, "rooms")
	}
}

func (cache *RoomCache) Unload(node *Room) {
	cache.Lock()
	defer cache.Unlock()
	cache.llPop(node)
	ok := node.Unload()
	if !ok {
		fmt.Print("Unload returned false, pushing node back")
		cache.llPush(node)
	}
}

func (cache *RoomCache) newRoom(roomID id.RoomID) *Room {
	node := NewRoom(roomID, cache)
	cache.Map[node.ID] = node
	return node
}
