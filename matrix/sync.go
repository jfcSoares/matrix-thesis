//Based on https://github.com/tulir/gomuks/blob/master/matrix/sync.go

package matrix

import (
	"sync"
	"thesgo/matrix/rooms"
	"time"

	"maunium.net/go/gomuks/debug"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type ThesgoSyncer struct {
	rooms             *rooms.RoomCache
	globalListeners   []mautrix.SyncHandler
	listeners         map[event.Type][]mautrix.EventHandler // event type to listeners array
	FirstSyncDone     bool
	InitDoneCallback  func()
	FirstDoneCallback func()
}

// NewThesgoSyncer returns an instantiated ThesgoSyncer
func NewThesgoSyncer(rooms *rooms.RoomCache) *ThesgoSyncer {
	return &ThesgoSyncer{
		rooms:           rooms,
		globalListeners: []mautrix.SyncHandler{},
		listeners:       make(map[event.Type][]mautrix.EventHandler),
		FirstSyncDone:   false,
		//Progress:        StubSyncingModal{},
	}
}

// ProcessResponse processes a Matrix sync response.
func (s *ThesgoSyncer) ProcessResponse(res *mautrix.RespSync, since string) (err error) {
	if since == "" {
		s.rooms.DisableUnloading()
	}
	debug.Print("Received sync response")
	//s.Progress.SetMessage("Processing sync response")
	steps := len(res.Rooms.Join) + len(res.Rooms.Invite) + len(res.Rooms.Leave)
	//s.Progress.SetSteps(steps + 2 + len(s.globalListeners))

	wait := &sync.WaitGroup{}
	callback := func() {
		wait.Done()
		//s.Progress.Step()
	}
	wait.Add(len(s.globalListeners))
	s.notifyGlobalListeners(res, since, callback)
	wait.Wait()

	s.processSyncEvents(nil, res.Presence.Events, mautrix.EventSourcePresence)
	//s.Progress.Step()
	s.processSyncEvents(nil, res.AccountData.Events, mautrix.EventSourceAccountData)
	//s.Progress.Step()

	wait.Add(steps)

	for roomID, roomData := range res.Rooms.Join {
		go s.processJoinedRoom(roomID, *roomData, callback)
	}

	for roomID, roomData := range res.Rooms.Invite {
		go s.processInvitedRoom(roomID, *roomData, callback)
	}

	for roomID, roomData := range res.Rooms.Leave {
		go s.processLeftRoom(roomID, *roomData, callback)
	}

	wait.Wait()
	//s.Progress.SetMessage("Finishing sync")

	if since == "" && s.InitDoneCallback != nil {
		s.InitDoneCallback()
		s.rooms.EnableUnloading()
	}
	if !s.FirstSyncDone && s.FirstDoneCallback != nil {
		s.FirstDoneCallback()
	}
	s.FirstSyncDone = true
	return
}

func (s *ThesgoSyncer) notifyGlobalListeners(res *mautrix.RespSync, since string, callback func()) {
	for _, listener := range s.globalListeners {
		go func(listener mautrix.SyncHandler) {
			listener(res, since)
			callback()
		}(listener)
	}
}

func (s *ThesgoSyncer) processJoinedRoom(roomID id.RoomID, roomData mautrix.SyncJoinedRoom, callback func()) {
	defer debug.Recover()
	room := s.rooms.GetOrCreate(roomID)
	room.UpdateSummary(roomData.Summary)
	s.processSyncEvents(room, roomData.State.Events, mautrix.EventSourceJoin|mautrix.EventSourceState)
	s.processSyncEvents(room, roomData.Timeline.Events, mautrix.EventSourceJoin|mautrix.EventSourceTimeline)
	s.processSyncEvents(room, roomData.Ephemeral.Events, mautrix.EventSourceJoin|mautrix.EventSourceEphemeral)
	s.processSyncEvents(room, roomData.AccountData.Events, mautrix.EventSourceJoin|mautrix.EventSourceAccountData)

	if len(room.PrevBatch) == 0 {
		room.PrevBatch = roomData.Timeline.PrevBatch
	}
	room.LastPrevBatch = roomData.Timeline.PrevBatch
	callback()
}

func (s *ThesgoSyncer) processInvitedRoom(roomID id.RoomID, roomData mautrix.SyncInvitedRoom, callback func()) {
	defer debug.Recover()
	room := s.rooms.GetOrCreate(roomID)
	room.UpdateSummary(roomData.Summary)
	s.processSyncEvents(room, roomData.State.Events, mautrix.EventSourceInvite|mautrix.EventSourceState)
	callback()
}

func (s *ThesgoSyncer) processLeftRoom(roomID id.RoomID, roomData mautrix.SyncLeftRoom, callback func()) {
	defer debug.Recover()
	room := s.rooms.GetOrCreate(roomID)
	room.HasLeft = true
	room.UpdateSummary(roomData.Summary)
	s.processSyncEvents(room, roomData.State.Events, mautrix.EventSourceLeave|mautrix.EventSourceState)
	s.processSyncEvents(room, roomData.Timeline.Events, mautrix.EventSourceLeave|mautrix.EventSourceTimeline)

	if len(room.PrevBatch) == 0 {
		room.PrevBatch = roomData.Timeline.PrevBatch
	}
	room.LastPrevBatch = roomData.Timeline.PrevBatch
	callback()
}

func (s *ThesgoSyncer) processSyncEvents(room *rooms.Room, events []*event.Event, source mautrix.EventSource) {
	for _, evt := range events {
		s.processSyncEvent(room, evt, source)
	}
}

func (s *ThesgoSyncer) processSyncEvent(room *rooms.Room, evt *event.Event, source mautrix.EventSource) {
	if room != nil {
		evt.RoomID = room.ID
	}
	// Ensure the type class is correct. It's safe to mutate since it's not a pointer.
	// Listeners are keyed by type structs, which means only the correct class will pass.
	switch {
	case evt.StateKey != nil:
		evt.Type.Class = event.StateEventType
	case source == mautrix.EventSourcePresence, source&mautrix.EventSourceEphemeral != 0:
		evt.Type.Class = event.EphemeralEventType
	case source&mautrix.EventSourceAccountData != 0:
		evt.Type.Class = event.AccountDataEventType
	case source == mautrix.EventSourceToDevice:
		evt.Type.Class = event.ToDeviceEventType
	default:
		evt.Type.Class = event.MessageEventType
	}

	err := evt.Content.ParseRaw(evt.Type)
	if err != nil {
		debug.Printf("Failed to unmarshal content of event %s (type %s) by %s in %s: %v\n%s", evt.ID, evt.Type.Repr(), evt.Sender, evt.RoomID, err, string(evt.Content.VeryRaw))
		// TODO might be good to let these pass to allow handling invalid events too
		return
	}

	if room != nil && evt.Type.IsState() {
		room.UpdateState(evt)
	}
	s.notifyListeners(source, evt)
}

// OnEventType allows callers to be notified when there are new events for the given event type.
// There are no duplicate checks.
func (s *ThesgoSyncer) OnEventType(eventType event.Type, callback mautrix.EventHandler) {
	_, exists := s.listeners[eventType]
	if !exists {
		s.listeners[eventType] = []mautrix.EventHandler{}
	}
	s.listeners[eventType] = append(s.listeners[eventType], callback)
}

func (s *ThesgoSyncer) OnSync(callback mautrix.SyncHandler) {
	s.globalListeners = append(s.globalListeners, callback)
}

func (s *ThesgoSyncer) notifyListeners(source mautrix.EventSource, evt *event.Event) {
	listeners, exists := s.listeners[evt.Type]
	if !exists {
		return
	}
	for _, fn := range listeners {
		fn(source, evt)
	}
}

// OnFailedSync always returns a 10 second wait period between failed /syncs, never a fatal error.
func (s *ThesgoSyncer) OnFailedSync(res *mautrix.RespSync, err error) (time.Duration, error) {
	debug.Printf("Sync failed: %v", err)
	return 10 * time.Second, nil
}

// GetFilterJSON returns a filter with a timeline limit of 50.
func (s *ThesgoSyncer) GetFilterJSON(_ id.UserID) *mautrix.Filter {
	stateEvents := []event.Type{
		event.StateMember,
		event.StateRoomName,
		event.StateTopic,
		event.StateCanonicalAlias,
		event.StatePowerLevels,
		event.StateTombstone,
		event.StateEncryption,
	}
	messageEvents := []event.Type{
		event.EventMessage,
		event.EventRedaction,
		event.EventEncrypted,
		//event.EventSticker,
		//event.EventReaction,
	}
	return &mautrix.Filter{
		Room: mautrix.RoomFilter{
			IncludeLeave: false,
			State: mautrix.FilterPart{
				LazyLoadMembers: true,
				Types:           stateEvents,
			},
			Timeline: mautrix.FilterPart{
				LazyLoadMembers: true,
				Types:           append(messageEvents, stateEvents...),
				Limit:           50,
			},
			Ephemeral: mautrix.FilterPart{
				Types: []event.Type{event.EphemeralEventTyping, event.EphemeralEventReceipt},
			},
			AccountData: mautrix.FilterPart{
				Types: []event.Type{event.AccountDataRoomTags},
			},
		},
		AccountData: mautrix.FilterPart{
			Types: []event.Type{event.AccountDataPushRules, event.AccountDataDirectChats},
		},
		Presence: mautrix.FilterPart{
			NotTypes: []event.Type{event.NewEventType("*")},
		},
	}
}
