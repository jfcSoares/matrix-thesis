package rooms

import (
	"encoding/json"
	"time"

	sync "github.com/sasha-s/go-deadlock"

	"maunium.net/go/gomuks/matrix/rooms"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type RoomNameSource int

const (
	UnknownRoomName RoomNameSource = iota
	MemberRoomName
	CanonicalAliasRoomName
	ExplicitRoomName
)

// RoomTag is a tag given to a specific room.
type RoomTag struct {
	// The name of the tag.
	Tag string
	// The order of the tag.
	Order json.Number
}

type UnreadMessage struct {
	EventID   id.EventID
	Counted   bool
	Highlight bool
}

type Member struct {
	event.MemberEventContent

	// The user who sent the membership event
	Sender id.UserID `json:"-"`
}

// Room represents a single Matrix room.
type Room struct {
	ID id.RoomID // The room ID.

	HasLeft bool // Whether or not the user has left the room.

	Encrypted bool // Whether or not the room is encrypted.

	// The first batch of events that has been fetched for this room.
	// Used for fetching additional history.
	PrevBatch string

	// The last_batch field from the most recent sync. Used for fetching member lists.
	LastPrevBatch string

	// The MXID of the user whose session this room was created for.
	SessionUserID id.UserID
	SessionMember *Member

	// The number of unread messages that were notified about.
	UnreadMessages   []UnreadMessage
	unreadCountCache *int
	highlightCache   *bool
	lastMarkedRead   id.EventID

	// Whether or not this room is marked as a direct chat.
	IsDirect  bool
	OtherUser id.UserID

	// List of tags given to this room.
	RawTags []RoomTag

	// Timestamp of previously received actual message.
	LastReceivedMessage time.Time

	// The lazy loading summary for this room.
	Summary mautrix.LazyLoadSummary

	// Whether or not the members for this room have been fetched from the server.
	MembersFetched bool

	// Room state cache.
	state map[event.Type]map[string]*event.Event

	// MXID -> Member cache calculated from membership events.
	memberCache map[id.UserID]*Member

	exMemberCache map[id.UserID]*Member

	// The first two non-SessionUserID members in the room. Calculated at
	// the same time as memberCache.
	firstMemberCache  *Member
	secondMemberCache *Member

	// The name of the room. Calculated from the state event name,
	// canonical_alias or alias or the member cache.
	NameCache string

	// The event type from which the name cache was calculated from.
	nameCacheSource RoomNameSource

	// The topic of the room. Directly fetched from the m.room.topic state event.
	topicCache string

	// The canonical alias of the room. Directly fetched from the m.room.canonical_alias state event.
	CanonicalAliasCache id.RoomAlias

	replacedCache bool // Whether or not the room has been tombstoned.

	replacedByCache *id.RoomID // The room ID that replaced this room.

	path string // Path for state store file.

	cache *rooms.RoomCache // Room cache object

	lock sync.RWMutex // Lock for state and other room stuff.

	// Pre/post un/load hooks
	preUnload  func() bool
	preLoad    func() bool
	postUnload func()
	postLoad   func()

	changed bool // Whether or not the room state has changed

	// Room state cache linked list.
	prev  *Room
	next  *Room
	touch int64
}

func (room *Room) getID() id.RoomID {
	return room.ID
}

func (room *Room) LoadedState() bool {
	return room.state != nil
}
