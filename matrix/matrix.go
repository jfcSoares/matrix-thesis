package matrix

import (
	"context"
	"errors"
	"fmt"
	"matrix/matrix/events"
	"os"
	"time"

	"github.com/rs/zerolog"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"maunium.net/go/gomuks/config"
	"maunium.net/go/gomuks/matrix"
	"maunium.net/go/gomuks/matrix/muksevt"
	"maunium.net/go/gomuks/matrix/rooms"
)

type ClientWrapper struct {
	client *mautrix.Client //the matrix client which communicates with the homeserver

	syncer *mautrix.DefaultSyncer //responsible for syncing data with server

	history *matrix.HistoryManager //responsible for storing event history

	crypto *crypto.OlmMachine //Main struct to handle Matrix E2EE

	config *config.Config // persist user account information and configurations

	logger zerolog.Logger

	//rooms *rooms.RoomCache //database for room information -> commented out, it is sitting on the config object now

	running bool

	stop chan bool
}

var MinSpecVersion = mautrix.SpecV11
var SkipVersionCheck = false

var (
	ErrNoHomeserver   = errors.New("no homeserver entered")
	ErrServerOutdated = errors.New("homeserver is outdated")
)

// NewWrapper creates a new ClientWrapper object for the given client instance.
func NewWrapper() *ClientWrapper {

	c := &ClientWrapper{
		//config: config.NewConfig() decide how to load directories into this
		logger:  NewWrapper().initLogger(),
		running: false,
	}

	return c
}

func (c *ClientWrapper) initLogger() zerolog.Logger {
	log := zerolog.New(os.Stderr).With().Timestamp().Logger()
	log = log.Level(zerolog.InfoLevel)

	return log
}

// initializes the client and connects to the specified homeserver
func (c *ClientWrapper) InitClient(isStartup bool) error {

	if c.Initialized() {
		c.Stop()
		c.client = nil
		c.crypto = nil
	}

	/*var mxid id.UserID
	var accessToken string
	if len(c.client.AccessToken) > 0 { //if a a client's credentials are still saved
		accessToken = c.client.AccessToken
		mxid = c.client.UserID
	} else {
		mxid = userID
	}*/ //once, or if, the sessions are persisted through several logins

	var err error
	c.client, err = mautrix.NewClient("https://lpgains.duckdns.org", "", "")
	if err != nil {
		c.logger.Error().Msg("failed to create mautrix client: " + err.Error())
		return fmt.Errorf("failed to create mautrix client: %w", err)
	}

	err = c.initCrypto()
	if err != nil {
		c.logger.Err(err).Msg("failed to initialize crypto")
		return fmt.Errorf("failed to initialize crypto: %w", err)
	}

	if c.history == nil {
		c.history, err = matrix.NewHistoryManager(c.config.HistoryPath)
		if err != nil {
			c.logger.Err(err).Msg("failed to initialize history")
			return fmt.Errorf("failed to initialize history: %w", err)
		}
	}

	/*allowInsecure := len(os.Getenv("CLIENT_ALLOW_INSECURE_CONNECTIONS")) > 0
	if allowInsecure {
		c.client.Client = &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		}
	}*/ //just in case, but probably will not be necessary

	/*if !SkipVersionCheck && (!isStartup || len(c.client.AccessToken) > 0) {
		fmt.Printf("Checking versions that %s supports.", c.client.HomeserverURL)
		resp, err := c.client.Versions()
		if err != nil {
			fmt.Print("Error checking supported versions:", err)
			return fmt.Errorf("failed to check server versions: %w", err)
		} else if !resp.ContainsGreaterOrEqual(MinSpecVersion) {
			fmt.Print("Server doesn't support modern spec versions.")
			bestVersionStr := "nothing"
			bestVersion := gomatrix.MustParseSpecVersion("r0.0.0")
			for _, ver := range resp.Versions {
				if ver.GreaterThan(bestVersion) {
					bestVersion = ver
					bestVersionStr = ver.String()
				}
			}
			return fmt.Errorf("%w (it only supports %s, while this client requires %s)", ErrServerOutdated, bestVersionStr, MinSpecVersion.String())
		} else {
			fmt.Print("Server supports modern spec versions")
		}
	}*/ //for posterity, but will probably not be required, since we know the properties of the server a priori

	c.stop = make(chan bool, 1)

	/*if len(accessToken) > 0 {
		go c.Start()
	}*/

	return nil
}

// Client returns the underlying matrix Client.
func (c *ClientWrapper) Client() *mautrix.Client {
	return c.client
}

func (c *ClientWrapper) IsStopped() chan bool {
	return c.stop
}

// Initialized returns whether or not the matrix client is initialized, i.e., has been instantiated
func (c *ClientWrapper) Initialized() bool {
	return c.client != nil
}

// Login sends a password login request with the given username and password.
func (c *ClientWrapper) Login(user, password string) error {
	resp, err := c.client.GetLoginFlows()
	if err != nil {
		c.logger.Error().Msg("could not check the login flows supported by the homeserver")
		return err
	}

	for _, flow := range resp.Flows {
		if flow.Type == "m.login.password" {
			return c.PasswordLogin(user, password)
		} else if flow.Type == "m.login.sso" {
			c.logger.Error().Msg("SSO login method is not supported")
			return fmt.Errorf("SSO login method is not supported")
		} else {
			c.logger.Error().Msg("login flow is not supported")
			return fmt.Errorf("Login flow is not supported")
		}

	}

	return nil
}

// Manual login
func (c *ClientWrapper) PasswordLogin(user, password string) error {

	resp, err := c.client.Login(&mautrix.ReqLogin{
		Type: "m.login.password",
		Identifier: mautrix.UserIdentifier{
			Type: "m.id.user",
			User: user,
		},
		Password:                 password,
		InitialDeviceDisplayName: "dev1",

		StoreCredentials:   true,
		StoreHomeserverURL: true,
	})

	if err != nil {
		c.logger.Error().Msg("could not login -> " + err.Error())
		return err
	}

	c.client.SetCredentials(resp.UserID, resp.AccessToken)
	c.concludeLogin(resp)

	return nil
}

// Concludes the login process, by assigning some last values to config fields
func (c *ClientWrapper) concludeLogin(resp *mautrix.RespLogin) {
	fmt.Println(resp.UserID + " = " + c.client.UserID)
	fmt.Println(resp.AccessToken + " = " + c.client.AccessToken)

	//Persist client credentials
	c.config.UserID = resp.UserID
	c.config.DeviceID = resp.DeviceID
	c.config.AccessToken = resp.AccessToken
	if resp.WellKnown != nil && len(resp.WellKnown.Homeserver.BaseURL) > 0 {
		c.config.HS = resp.WellKnown.Homeserver.BaseURL
	}

	c.config.Save()
	//go c.Start()
}

func (c *ClientWrapper) Logout() {
	fmt.Println("Logging out...")
	c.logger.Info().Msg("Logging out...")
	c.client.Logout()
	c.Stop()
	c.config.DeleteSession()
	c.client.ClearCredentials()
	c.client = nil
	c.crypto = nil
}

func (c *ClientWrapper) Start() {

	//c.OnLogin() Initialize the syncer

	if c.client == nil {
		return
	}

	fmt.Print("Starting sync...")
	c.logger.Info().Msg("Starting sync...")
	c.running = true
	c.client.StreamSyncMinAge = 30 * time.Minute
	for {
		select {
		case <-c.stop:
			fmt.Print("Stopping sync...")
			c.logger.Info().Msg("Stopping sync...")
			c.running = false
			return
		default:
			if err := c.client.Sync(); err != nil {
				if errors.Is(err, mautrix.MUnknownToken) {
					fmt.Print("Sync() errored with ", err, " -> logging out")
					c.logger.Error().Msg("Access token was not recognized -> logging out")
					c.Logout()
				} else {
					fmt.Print("Sync() errored", err)
					c.logger.Error().Msg("Sync() call errored with: " + err.Error())
				}
			} else {
				fmt.Print("Sync() returned without error")
				c.logger.Info().Msg("Sync() call returned successfully")
				c.Logout() //ONLY FOR TESTING
			}
		}
	}
}

// Stop stops the Matrix syncer.
func (c *ClientWrapper) Stop() {
	if c.running {
		fmt.Print("Stopping Matrix client...")
		c.logger.Info().Msg("Stopping Matrix client...")
		select {
		case c.stop <- true:
		default:
		}
		c.client.StopSync()
		fmt.Print("Closing history manager...")
		err := c.history.Close()
		if err != nil {
			fmt.Print("Error closing history manager:", err)
		}
		c.history = nil

		if c.crypto != nil {
			c.logger.Info().Msg("Flushing crypto store")
			err := c.crypto.FlushStore()
			if err != nil {
				c.logger.Error().Msg("Error on flushing the crypto store: " + err.Error())
			}
		}
	}
}

// OnLogin initializes the syncer and updates the room list.
/*func (c *ClientWrapper) OnLogin() {
	//c.cryptoOnLogin()
	//c.ui.OnLogin()

	c.client.Store = c.config

	c.logger.Info().Msg("Initializing syncer")
	c.syncer = mautrix.NewDefaultSyncer()
	if c.crypto != nil {
		c.syncer.OnSync(c.crypto.ProcessSyncResponse)
		c.syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
			// Don't spam the crypto module with member events of an initial sync
			// TODO invalidate all group sessions when clearing cache?
			if c.config.AuthCache.InitialSyncDone {
				c.crypto.HandleMemberEvent(evt)
			}
		})
		c.syncer.OnEventType(event.EventEncrypted, c.HandleEncrypted)
	} else {
		c.syncer.OnEventType(event.EventEncrypted, c.HandleEncryptedUnsupported)
	}
	c.syncer.OnEventType(event.EventMessage, c.HandleMessage)
	c.syncer.OnEventType(event.EventSticker, c.HandleMessage)
	c.syncer.OnEventType(event.EventReaction, c.HandleMessage)
	c.syncer.OnEventType(event.EventRedaction, c.HandleRedaction)
	c.syncer.OnEventType(event.StateAliases, c.HandleMessage)
	c.syncer.OnEventType(event.StateCanonicalAlias, c.HandleMessage)
	c.syncer.OnEventType(event.StateTopic, c.HandleMessage)
	c.syncer.OnEventType(event.StateRoomName, c.HandleMessage)
	c.syncer.OnEventType(event.StateMember, c.HandleMembership)
	c.syncer.OnEventType(event.EphemeralEventReceipt, c.HandleReadReceipt)
	c.syncer.OnEventType(event.EphemeralEventTyping, c.HandleTyping)
	c.syncer.OnEventType(event.AccountDataDirectChats, c.HandleDirectChatInfo)
	c.syncer.OnEventType(event.AccountDataPushRules, c.HandlePushRules)
	c.syncer.OnEventType(event.AccountDataRoomTags, c.HandleTag)
	//TODO: Add custom event handler for offline comms maybe?
	/*c.syncer.InitDoneCallback = func() {
		fmt.Print("Initial sync done")
		c.config.AuthCache.InitialSyncDone = true
		fmt.Print("Updating title caches")
		for _, room := range c.config.Rooms.Map {
			room.GetTitle()
		}
		fmt.Print("Cleaning cached rooms from memory")
		c.config.Rooms.ForceClean()
		fmt.Print("Saving all data")
		c.config.SaveAll()
		debug.Print("Adding rooms to UI")
		c.ui.MainView().SetRooms(c.config.Rooms)
		c.ui.Render()
		// The initial sync can be a bit heavy, so we force run the GC here
		// after cleaning up rooms from memory above.
		fmt.Println("Running GC")
		runtime.GC()
		dbg.FreeOSMemory()
	}*
	//possibly some interface code as well later?

	c.client.Syncer = c.syncer

	fmt.Println("OnLogin() done.")
}*/

//*************************** ROOMS *******************************//

// Attempts to create a new room with the given name, topic, and an invited users list
func (c *ClientWrapper) NewRoom(roomName string, topic string, inviteList []id.UserID) (*rooms.Room, error) {
	resp, err := c.client.CreateRoom(&mautrix.ReqCreateRoom{
		Preset: "trusted_private_chat",
		Name:   roomName,
		Topic:  topic,
		Invite: inviteList,
	})

	if err != nil {
		return nil, err
	}

	fmt.Println("Created room with ID: " + resp.RoomID)
	room := c.GetOrCreateRoom(resp.RoomID)
	c.logger.Info().Msg("Created room with ID:" + room.ID.String())

	return room, nil
}

// Stops a user from participating in a given room, but it may still be able to retrieve its history
// if it rejoins the same room
func (c *ClientWrapper) ExitRoom(roomID id.RoomID, reason string) error {
	var resp *mautrix.RespLeaveRoom
	var err error

	if reason != "" {
		resp, err = c.client.LeaveRoom(roomID, &mautrix.ReqLeave{
			Reason: reason,
		})

	} else {
		resp, err = c.client.LeaveRoom(roomID)
	}

	if err != nil {
		fmt.Errorf("could not leave room: %w", err)
		c.logger.Error().Msg("Could not leave room due to: " + err.Error())
		return err
	}

	node := c.GetOrCreateRoom(roomID)
	node.HasLeft = true
	node.Unload()
	fmt.Println(resp)
	fmt.Println("Left room with ID: " + roomID)
	c.logger.Info().Msg("Left room with ID: " + roomID.String())

	return nil
}

// After this function is called, a user will no longer be able to retrieve history for the given room.
// If all users on a homeserver forget a room, the room is eligible for deletion from that homeserver.
func (c *ClientWrapper) ForgetRoom(roomID id.RoomID) error {
	resp, err := c.client.ForgetRoom(roomID)

	if err != nil {
		c.logger.Error().Err(err).Msg("could not forget room")
		return err
	}

	fmt.Println(resp)
	fmt.Println("Forgot room with ID: " + roomID)
	c.logger.Info().Msg("Forgot room with ID: " + roomID.String())
	return nil
}

// Invites the given user to the room corresponding to the provided roomID
func (c *ClientWrapper) InviteUser(roomID id.RoomID, reason, user string) error {
	_, err := c.client.InviteUser(roomID, &mautrix.ReqInviteUser{
		Reason: reason,
		UserID: id.UserID(user),
	})

	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not invite user " + user + " to room with ID: " + roomID.String())
		return err
	}

	fmt.Println("Successfully Invited " + user + " to the room with id: " + roomID.String())
	c.logger.Info().Msg("Successfully Invited " + user + " to the room with id: " + roomID.String())

	return nil
}

// Lists the rooms the user is currently joined into
func (c *ClientWrapper) RoomsJoined() ([]id.RoomID, error) {
	resp, err := c.client.JoinedRooms()

	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not list the rooms the user is joined to")
		return nil, err
	} else {
		for i := 0; i < len(resp.JoinedRooms); i++ {
			fmt.Println(resp.JoinedRooms[i])
			//first indexes are the most recent rooms
		}
	}

	return resp.JoinedRooms, err
}

// Joins a room with the given room or alias, through the specified server in the arguments
func (c *ClientWrapper) JoinRoom(roomIdOrAlias, server string, content interface{}) (*rooms.Room, error) {
	//maybe add a check to see if content is nil
	resp, err := c.client.JoinRoom(roomIdOrAlias, server, content)

	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not join room with the given id or alias")
		return nil, err
	}

	room := c.GetOrCreateRoom(resp.RoomID)
	room.HasLeft = false
	fmt.Println("Successfully joined room with id: " + roomIdOrAlias)
	c.logger.Info().Msg("Successfully joined room with ID: " + roomIdOrAlias)

	return room, nil
}

// Retrieves the list of members of the given room
func (c *ClientWrapper) JoinedMembers(roomID id.RoomID) error {
	resp, err := c.client.JoinedMembers(roomID)

	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not list the members of the given room")
		return err
	} else {
		for key := range resp.Joined { //print out the room members' display names
			fmt.Println(key)
		}
	}

	return nil
}

func (c *ClientWrapper) FetchMembers(room *rooms.Room) error {
	fmt.Print("Fetching member list for", room.ID)
	members, err := c.client.Members(room.ID, mautrix.ReqMembers{At: room.LastPrevBatch})
	if err != nil {
		c.logger.Err(err).Msg("Could not fetch members of room " + room.ID.String())
		return err
	}
	fmt.Printf("Fetched %d members for %s", len(members.Chunk), room.ID)
	for _, evt := range members.Chunk {
		err := evt.Content.ParseRaw(evt.Type)
		if err != nil {
			fmt.Printf("Failed to parse member event of %s: %v", evt.GetStateKey(), err)
			c.logger.Err(err).Msg("Failed to parse member event of " + evt.GetStateKey())
			continue
		}
		room.UpdateState(evt)
	}
	room.MembersFetched = true
	return nil
}

// GetHistory fetches room history.
func (c *ClientWrapper) GetHistory(room *rooms.Room, limit int, dbPointer uint64) ([]*muksevt.Event, uint64, error) {
	events, newDBPointer, err := c.history.Load(room, limit, dbPointer) //tries to obtain event history of the given room locally
	if err != nil {
		c.logger.Err(err).Msg("Could not load events of room " + room.ID.String() + " from local cache")
		return nil, dbPointer, err
	}

	if len(events) > 0 {
		fmt.Printf("Loaded %d events for %s from local cache", len(events), room.ID)
		c.logger.Info().Msg("Loaded " + string(len(events)) + "from local cache of room with ID: " + room.ID.String())
		return events, newDBPointer, nil
	}

	resp, err := c.client.Messages(room.ID, room.PrevBatch, "", 'b', nil, limit) //otherwise fetch history from server
	if err != nil {
		c.logger.Err(err).Msg("Could not load events of room " + room.ID.String() + " from homeserver")
		return nil, dbPointer, err
	}

	fmt.Printf("Loaded %d events for %s from server from %s to %s", len(resp.Chunk), room.ID, resp.Start, resp.End)
	for i, evt := range resp.Chunk {
		err := evt.Content.ParseRaw(evt.Type)
		if err != nil {
			fmt.Printf("Failed to unmarshal content of event %s (type %s) by %s in %s: %v\n%s", evt.ID, evt.Type.Repr(), evt.Sender, evt.RoomID, err, string(evt.Content.VeryRaw))
			c.logger.Err(err).Msg("Failed to unmarshal content of event " + evt.ID.String() + " (type " + evt.Type.Repr() + ") by " + string(evt.Sender) + " in " + string(evt.RoomID) + " with content:" + string(evt.Content.VeryRaw))
		}

		if evt.Type == event.EventEncrypted {
			if c.crypto == nil {
				evt.Type = muksevt.EventEncryptionUnsupported
				origContent, _ := evt.Content.Parsed.(*event.EncryptedEventContent)
				evt.Content.Parsed = muksevt.EncryptionUnsupportedContent{Original: origContent}
			} else {
				decrypted, err := c.crypto.DecryptMegolmEvent(context.TODO(), evt)
				if err != nil {
					fmt.Printf("Failed to decrypt event %s: %v", evt.ID, err)
					c.logger.Err(err).Msg("Failed to decrypt event " + evt.ID.String())
					evt.Type = muksevt.EventBadEncrypted
					origContent, _ := evt.Content.Parsed.(*event.EncryptedEventContent)
					evt.Content.Parsed = &muksevt.BadEncryptedContent{
						Original: origContent,
						Reason:   err.Error(),
					}
				} else {
					resp.Chunk[i] = decrypted
				}
			}
		}
	}

	//update the local cache for the given room
	for _, evt := range resp.State {
		room.UpdateState(evt)
	}
	room.PrevBatch = resp.End
	c.config.Rooms.Put(room)
	if len(resp.Chunk) == 0 {
		return []*muksevt.Event{}, dbPointer, nil
	}
	// TODO newDBPointer isn't accurate in this case yet, fix later
	events, newDBPointer, err = c.history.Prepend(room, resp.Chunk) //update event history
	if err != nil {
		return nil, dbPointer, err
	}
	return events, dbPointer, nil
}

// Fetches a specific event of the given room
func (c *ClientWrapper) GetEvent(room *rooms.Room, eventID id.EventID) (*muksevt.Event, error) {
	evt, err := c.history.Get(room, eventID) //First tries to obtain the event from the local cache
	if err != nil && err != matrix.EventNotFoundError {
		fmt.Printf("Failed to get event %s from local cache: %v", eventID, err)
		c.logger.Err(err).Msg("Failed to get event " + eventID.String() + " from local cache")
	} else if evt != nil {
		fmt.Printf("Found event %s in local cache", eventID)
		c.logger.Info().Msg("Found event " + eventID.String() + " in local cache")
		return evt, err
	}

	mxEvent, err := c.client.GetEvent(room.ID, eventID) //Otherwise ask the server for it
	if err != nil {
		c.logger.Err(err).Msg("Could not get event")
		return nil, err
	}

	err = mxEvent.Content.ParseRaw(mxEvent.Type)
	if err != nil {
		c.logger.Err(err).Msg("Failed to unmarshal content of event " + evt.ID.String() + " (type " + evt.Type.Repr() + ") by " + string(evt.Sender) + " in " + string(evt.RoomID) + " with content:" + string(evt.Content.VeryRaw))
		return nil, err
	}
	fmt.Printf("Loaded event %s from server", eventID)
	c.logger.Info().Msg("Loaded event " + eventID.String() + " from server")
	return muksevt.Wrap(mxEvent), nil
}

// GetOrCreateRoom gets the room instance stored in the session.
func (c *ClientWrapper) GetOrCreateRoom(roomID id.RoomID) *rooms.Room {
	return c.config.Rooms.GetOrCreate(roomID)
}

// GetRoom gets the room instance stored in the session.
func (c *ClientWrapper) GetRoom(roomID id.RoomID) *rooms.Room {
	return c.config.Rooms.Get(roomID)
}

//*************************** EVENTS *******************************//

// Sends a message event into a room
func (c *ClientWrapper) SendMessageEvent(evt *events.Event) (id.EventID, error) {
	room := c.GetRoom(evt.RoomID)
	if room != nil && room.Encrypted && c.crypto != nil && evt.Type != event.EventReaction {
		encrypted, err := c.crypto.EncryptMegolmEvent(context.TODO(), evt.RoomID, evt.Type, &evt.Content)
		if err != nil {
			if isBadEncryptError(err) {
				c.logger.Error().Err(err).Msg("Could not encrypt the specified event")
				return "", err
			}
			fmt.Print("Got", err, "while trying to encrypt message, sharing group session and trying again...")
			err = c.crypto.ShareGroupSession(context.TODO(), room.ID, room.GetMemberList())
			if err != nil {
				c.logger.Error().Err(err).Msg("Could not share the group session successfully")
				return "", err
			}
			encrypted, err = c.crypto.EncryptMegolmEvent(context.TODO(), evt.RoomID, evt.Type, &evt.Content)
			if err != nil {
				c.logger.Error().Err(err).Msg("Could not encrypt the specified event")
				return "", err
			}
		}
		evt.Type = event.EventEncrypted
		evt.Content = event.Content{Parsed: encrypted}
	}

	resp, err := c.client.SendMessageEvent(evt.RoomID, evt.Type, &evt.Content, mautrix.ReqSendEvent{TransactionID: evt.Unsigned.TransactionID})
	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not send message event")
		return "", err
	}

	fmt.Println("Sent message with event ID: " + resp.EventID)
	c.logger.Info().Msg("Sent message with event ID: " + resp.EventID.String())
	return resp.EventID, nil
}

// Sends a state event into a room
func (c *ClientWrapper) SendStateEvent(evt *events.Event) (id.EventID, error) {
	//TODO: Encryption flow before sending the event

	resp, err := c.client.SendStateEvent(evt.RoomID, evt.Type, *evt.StateKey, &evt.Content)
	if err != nil {
		c.logger.Error().Err(err).Msg("could not send the specified event")
		return "", err
	}

	c.logger.Info().Msg("Sent message with event ID: " + resp.EventID.String())
	return resp.EventID, nil
}

// Sends a read receipt regarding the event in the arguments
func (c *ClientWrapper) SendReadReceipt(evt *events.Event) error {
	err := c.client.SendReceipt(evt.RoomID, evt.ID, event.ReceiptTypeRead, nil)

	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not send read receipt")
		return err
	}

	return nil
}

// Get the state events for the current state of the given room
func (c *ClientWrapper) GetState(roomID id.RoomID) (*mautrix.RoomStateMap, error) {
	resp, err := c.client.State(roomID)

	if err != nil {
		c.logger.Error().Err(err).Msg("could not get state for the given room")
		return nil, err
	}

	return &resp, nil
}

/*func (c *ClientWrapper) SendToDevice(eventType event.Type, req *mautrix.ReqSendToDevice) (*mautrix.RespSendToDevice, error) {
	probably will not need this
}*/

//****************** EVENT HANDLERS *********************//

// HandleMessage is the event handler for the m.room.message timeline event.
func (c *ClientWrapper) HandleMessage(source mautrix.EventSource, mxEvent *event.Event) {
	room := c.GetOrCreateRoom(mxEvent.RoomID)
	if source&mautrix.EventSourceLeave != 0 {
		room.HasLeft = true
		return
	} else if source&mautrix.EventSourceState != 0 {
		return
	}

	events, err := c.history.Append(room, []*event.Event{mxEvent}) //add the newly-received event to room history
	if err != nil {
		fmt.Printf("Failed to add event %s to history: %v", mxEvent.ID, err)
		c.logger.Err(err).Msg("Failed to add event " + mxEvent.ID.String() + " to history")
	}

	evt := events[0]
	if !c.config.AuthCache.InitialSyncDone {
		room.LastReceivedMessage = time.Unix(evt.Timestamp/1000, evt.Timestamp%1000*1000)
		return
	}

	c.logger.Info().
		Str("sender", evt.Sender.String()).
		Str("type", evt.Type.String()).
		Str("id", evt.ID.String()).
		Str("body", evt.Content.AsMessage().Body).
		Msg("Received message")

	//Possivelmente fazer alguma coisa com o conteudo da mensagem (se for um alerta de intruso por exemplo)

	//Açoes para o UI talvez

}

func (c *ClientWrapper) HandleEncrypted(source mautrix.EventSource, mxEvent *event.Event) {
	evt, err := c.crypto.DecryptMegolmEvent(context.TODO(), mxEvent)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to decrypt event contents")
		mxEvent.Type = muksevt.EventBadEncrypted
		origContent, _ := mxEvent.Content.Parsed.(*event.EncryptedEventContent)
		mxEvent.Content.Parsed = &muksevt.BadEncryptedContent{
			Original: origContent,
			Reason:   err.Error(),
		}
		c.HandleMessage(source, mxEvent)
		return
	}
	if evt.Type.IsInRoomVerification() {
		err := c.crypto.ProcessInRoomVerification(evt)
		if err != nil {
			c.logger.Error().Msg("[Crypto/Error] Failed to process in-room verification event " + evt.ID.String() + " of type " + evt.Type.String() + ":" + err.Error())

		} else {
			c.logger.Info().Msg("[Crypto/Debug] Processed in-room verification event " + evt.ID.String() + " of type " + evt.Type.String())
		}
	} else {
		c.HandleMessage(source, evt)
	}
}

func (c *ClientWrapper) HandleMembership(source mautrix.EventSource, evt *event.Event) {
	hasLeft := source&mautrix.EventSourceLeave != 0       //if the user has left the room
	isTimeline := source&mautrix.EventSourceTimeline != 0 //if it is a timeline event
	if hasLeft {
		c.GetOrCreateRoom(evt.RoomID).HasLeft = true
	}
	isNonTimelineLeave := hasLeft && !isTimeline
	if !c.config.AuthCache.InitialSyncDone && isNonTimelineLeave {
		return
	} else if evt.StateKey != nil && id.UserID(*evt.StateKey) == c.config.UserID {
		c.processOwnMembershipChange(evt)
	} else if !isTimeline && (!c.config.AuthCache.InitialSyncDone || hasLeft) {
		// We don't care about other users' membership events in the initial sync or chats we've left.
		return
	}

	c.HandleMessage(source, evt)
}

func (c *ClientWrapper) processOwnMembershipChange(evt *event.Event) {
	membership := evt.Content.AsMember().Membership
	prevMembership := event.MembershipLeave
	if evt.Unsigned.PrevContent != nil {
		prevMembership = evt.Unsigned.PrevContent.AsMember().Membership
	}
	fmt.Printf("Processing own membership change: %s->%s in %s", prevMembership, membership, evt.RoomID)
	if membership == prevMembership {
		return
	}
	room := c.GetRoom(evt.RoomID)
	switch membership {
	case "join":
		room.HasLeft = false
		/*if c.config.AuthCache.InitialSyncDone {
			c.ui.MainView().UpdateTags(room)
		}*/
		fallthrough
	case "invite":
		/*if c.config.AuthCache.InitialSyncDone {
			c.ui.MainView().AddRoom(room)
		}*/
	case "leave":
	case "ban":
		/*if c.config.AuthCache.InitialSyncDone {
			c.ui.MainView().RemoveRoom(room)
		}*/
		room.HasLeft = true
		room.Unload()
	default:
		return
	}
}
