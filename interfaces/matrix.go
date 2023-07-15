package ifc

import (
	"thesgo/matrix/mxevents"
	"thesgo/matrix/rooms"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

type MatrixContainer interface {
	Client() *mautrix.Client
	//Preferences() *config.UserPreferences
	InitClient(isStartup bool) error
	Initialized() bool

	Start()
	Stop()

	Login(user, password string) error
	Logout()
	//UIAFallback(authType mautrix.AuthType, sessionID string) error

	SendEvent(evt *mxevents.Event) (id.EventID, error)
	SendStateEvent(evt *mxevents.Event) (id.EventID, error)
	//Redact(roomID id.RoomID, eventID id.EventID, reason string) error
	//MarkRead(roomID id.RoomID, eventID id.EventID)
	JoinRoom(roomID id.RoomID, server string) (*rooms.Room, error)
	ExitRoom(roomID id.RoomID, reason string) error
	NewRoom(roomName string, topic string, inviteList []id.UserID) (*rooms.Room, error)
	ForgetRoom(roomID id.RoomID) error
	RoomsJoined() ([]id.RoomID, error)
	InviteUser(roomID id.RoomID, reason, user string) error

	FetchMembers(room *rooms.Room) error
	JoinedMembers(roomID id.RoomID) error //not sure if this is better than fetchMembers
	GetHistory(room *rooms.Room, limit int, dbPointer uint64) ([]*mxevents.Event, uint64, error)
	GetEvent(room *rooms.Room, eventID id.EventID) (*mxevents.Event, error)
	GetRoom(roomID id.RoomID) *rooms.Room
	GetOrCreateRoom(roomID id.RoomID) *rooms.Room

	//Crypto() Crypto Probaby will not need to define an interface for crypto ops, here just in case
}
