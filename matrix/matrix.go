// Based on https://github.com/tulir/gomuks/blob/master/matrix/matrix.go
package matrix

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	dbg "runtime/debug"
	"strconv"
	"sync"
	"time"

	"thesgo/config"
	"thesgo/debug"
	"thesgo/matrix/mxevents"
	"thesgo/matrix/rooms"
	"thesgo/offline"

	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"golang.org/x/exp/slices"

	"github.com/libp2p/go-libp2p"
	cryp "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	libp2ptls "github.com/libp2p/go-libp2p/p2p/security/tls"

	"github.com/rs/zerolog"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type ClientWrapper struct {
	client *mautrix.Client //the matrix client which communicates with the homeserver

	syncer *ThesgoSyncer //responsible for syncing data with server

	history *HistoryManager //responsible for storing event history

	crypto *crypto.OlmMachine //Main struct to handle Matrix E2EE

	config *config.Config // persist user account information and configurations

	logger zerolog.Logger

	running bool

	disconnected bool

	stop chan bool

	sendOff chan offlineData
}

var MinSpecVersion = mautrix.SpecV11
var SkipVersionCheck = false

var (
	ErrNoHomeserver   = errors.New("no homeserver entered")
	ErrServerOutdated = errors.New("homeserver is outdated")
)

// NewWrapper creates a new ClientWrapper object for the given client instance.
func NewWrapper(conf *config.Config) *ClientWrapper {

	c := &ClientWrapper{
		config:       conf,
		running:      false,
		disconnected: false,
	}

	c.initLogger()
	return c
}

func (c *ClientWrapper) initLogger() zerolog.Logger {
	return *debug.Initialize()
}

// initializes the client and connects to the specified homeserver
func (c *ClientWrapper) InitClient(isStartup bool) error {

	if c.Initialized() {
		c.Stop()
		c.client = nil
		c.crypto = nil
	}

	//if client session was persisted
	var mxid id.UserID
	var accessToken string
	if len(c.config.AccessToken) > 0 {
		accessToken = c.config.AccessToken
		mxid = c.config.UserID
	}
	fmt.Println(mxid.String() + ", " + accessToken)

	var err error
	if mxid.String() != "" && len(accessToken) > 0 {
		c.client, err = mautrix.NewClient("https://lpgains.duckdns.org", mxid, accessToken)
	} else {
		c.client, err = mautrix.NewClient("https://lpgains.duckdns.org", "", "")
	}

	if err != nil {
		//c.offline = true, might be unsafe -> client might fail due to wrong access token and not by losing conn
		c.logger.Error().Msg("failed to create mautrix client: " + err.Error())
		return fmt.Errorf("failed to create mautrix client: %w", err)
	}

	c.disconnected = false
	c.client.Log = c.initLogger()
	c.client.DeviceID = c.config.DeviceID

	err = c.initCrypto()
	if err != nil {
		c.logger.Err(err).Msg("failed to initialize crypto")
		return fmt.Errorf("failed to initialize crypto: %w", err)
	}

	if c.history == nil {
		c.history, err = NewHistoryManager(c.config.HistoryPath)
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

	if !SkipVersionCheck && (!isStartup || len(c.client.AccessToken) > 0) { //sanity check
		c.logger.Info().Msg("Checking versions that " + c.client.HomeserverURL.String() + " supports.")
		resp, err := c.client.Versions()
		if err != nil {
			c.logger.Err(err).Msg("Error checking supported versions")
			return fmt.Errorf("failed to check server versions: %w", err)
		} else if !resp.ContainsGreaterOrEqual(MinSpecVersion) {
			c.logger.Info().Msg("Server doesn't support modern spec versions.")
			bestVersionStr := "nothing"
			bestVersion := mautrix.MustParseSpecVersion("r0.0.0")
			for _, ver := range resp.Versions {
				if ver.GreaterThan(bestVersion) {
					bestVersion = ver
					bestVersionStr = ver.String()
				}
			}
			return fmt.Errorf("%w (it only supports %s, while this client requires %s)", ErrServerOutdated, bestVersionStr, MinSpecVersion.String())
		} else {
			c.logger.Info().Msg("Server supports modern spec versions")
		}
	}

	c.stop = make(chan bool, 1)
	c.sendOff = make(chan offlineData, 1) //possibly making this a buffered channel might be good

	if len(accessToken) > 0 {
		go c.Start()
	}
	go c.runOffline() //start routine to open a host for listening and/or sending offline comms

	return nil
}

// Client returns the underlying matrix Client.
func (c *ClientWrapper) Client() *mautrix.Client {
	return c.client
}

func (c *ClientWrapper) Crypto() *crypto.OlmMachine {
	return c.crypto
}

func (c *ClientWrapper) IsStopped() chan bool {
	return c.stop
}

func (c *ClientWrapper) IsOffline() bool {
	return c.disconnected
}

// Initialized returns whether or not the matrix client is initialized, i.e., has been instantiated
func (c *ClientWrapper) Initialized() bool {
	return c.running
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

	c.concludeLogin(resp)

	return nil
}

// Concludes the login process, by assigning some last values to config fields
func (c *ClientWrapper) concludeLogin(resp *mautrix.RespLogin) {

	//Persist client credentials
	c.config.UserID = resp.UserID
	c.config.DeviceID = resp.DeviceID
	c.config.AccessToken = resp.AccessToken
	if resp.WellKnown != nil && len(resp.WellKnown.Homeserver.BaseURL) > 0 {
		c.config.Homeserver = resp.WellKnown.Homeserver.BaseURL
	}

	c.config.Save()
	go c.Start()
}

func (c *ClientWrapper) Logout() {
	c.logger.Info().Msg("Logging out...")
	c.client.Logout()
	c.Stop()
	c.config.DeleteSession()
	c.client.ClearCredentials()
	c.client = nil
	c.crypto = nil
}

func (c *ClientWrapper) Start() {
	c.cryptoOnLogin() //get crypto store from files if they exist
	c.OnLogin()       //Initialize the syncer

	if c.client == nil {
		return
	}

	c.logger.Info().Msg("Starting sync...")
	c.running = true
	c.client.StreamSyncMinAge = 30 * time.Minute //syncs with the server every 30min
	for {
		select {
		case <-c.stop:
			c.logger.Info().Msg("Stopping sync...")
			c.running = false
			return
		default:
			if err := c.client.Sync(); err != nil {
				if errors.Is(err, mautrix.MUnknownToken) {
					c.logger.Error().Msg("Access token was not recognized -> logging out")
					c.Logout()
				} else {
					c.logger.Error().Msg("Sync() call errored with: " + err.Error())
				}
			} else {
				//fmt.Print("Sync() returned without error")
				c.logger.Info().Msg("Sync() call returned successfully")
				//c.Logout() //ONLY FOR TESTING
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
			c.logger.Err(err).Msg("Error closing history manager")
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
func (c *ClientWrapper) OnLogin() {
	c.cryptoOnLogin()
	//c.ui.OnLogin()

	c.client.Store = c.config

	c.logger.Info().Msg("Initializing syncer")
	//Instantiate syncer and assign event handlers to corresponding event types
	c.syncer = NewThesgoSyncer(c.config.Rooms)
	if c.crypto != nil {
		c.syncer.OnSync(c.crypto.ProcessSyncResponse)
		c.syncer.OnEventType(event.StateMember, func(source mautrix.EventSource, evt *event.Event) {
			// Don't spam the crypto module with member events of an initial sync
			// TODO invalidate all group sessions when clearing cache?
			if c.config.AuthCache.InitialSyncDone {
				c.crypto.HandleMemberEvent(source, evt)
			}
		})
		c.syncer.OnEventType(event.EventEncrypted, c.HandleEncrypted)
	} else {
		c.syncer.OnEventType(event.EventEncrypted, c.HandleEncryptedUnsupported)
	}
	c.syncer.OnEventType(event.EventMessage, c.HandleMessage)
	c.syncer.OnEventType(event.EventSticker, c.HandleMessage)
	c.syncer.OnEventType(event.EventReaction, c.HandleMessage)
	//c.syncer.OnEventType(event.EventRedaction, c.HandleRedaction)
	c.syncer.OnEventType(event.StateAliases, c.HandleMessage)
	c.syncer.OnEventType(event.StateCanonicalAlias, c.HandleMessage)
	c.syncer.OnEventType(event.StateTopic, c.HandleMessage)
	c.syncer.OnEventType(event.StateRoomName, c.HandleMessage)
	c.syncer.OnEventType(event.StateMember, c.HandleMembership)
	c.syncer.OnEventType(event.StateEncryption, c.HandleRoomEncryption)
	c.syncer.OnEventType(event.EphemeralEventReceipt, c.HandleReadReceipt)
	/*c.syncer.OnEventType(event.EphemeralEventTyping, c.HandleTyping)
	c.syncer.OnEventType(event.AccountDataDirectChats, c.HandleDirectChatInfo)
	c.syncer.OnEventType(event.AccountDataPushRules, c.HandlePushRules)
	c.syncer.OnEventType(event.AccountDataRoomTags, c.HandleTag)*/
	//commented out the handlers for unnecessary features for now
	//TODO: Add custom event handler for offline comms maybe?
	c.syncer.InitDoneCallback = func() { //once first sync is done
		c.logger.Info().Msg("Initial sync done")
		c.config.AuthCache.InitialSyncDone = true
		c.logger.Info().Msg("Updating title caches")
		for _, room := range c.config.Rooms.Map {
			room.GetTitle()
		}
		c.logger.Info().Msg("Cleaning cached rooms from memory")
		c.config.Rooms.ForceClean()
		c.logger.Info().Msg("Saving all data")
		c.config.SaveAll()
		//fmt.Print("Adding rooms to UI")
		//c.ui.MainView().SetRooms(c.config.Rooms)
		//c.ui.Render()
		// The initial sync can be a bit heavy, so we force run the GC here
		// after cleaning up rooms from memory above.
		c.logger.Info().Msg("Running GC")
		runtime.GC()
		dbg.FreeOSMemory()
	}
	//possibly some interface code as well later?

	c.client.Syncer = c.syncer

	fmt.Println("OnLogin() done.")
}

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

	//Right after room creation, enable encryption for that room
	evt := &mxevents.Event{
		Event: &event.Event{
			Type:   event.StateEncryption,
			RoomID: room.ID,
			Content: event.Content{Parsed: &event.EncryptionEventContent{
				Algorithm:              id.AlgorithmMegolmV1,
				RotationPeriodMillis:   604800000, //for now use default session rotation
				RotationPeriodMessages: 100,
			}},
		},
	}

	c.SendStateEvent(evt)

	return room, nil
}

// Stops a user from participating in a given room, but it may still be able to retrieve its history
// if it rejoins the same room
func (c *ClientWrapper) ExitRoom(roomID id.RoomID, reason string) error {

	var err error

	if reason != "" {
		_, err = c.client.LeaveRoom(roomID, &mautrix.ReqLeave{
			Reason: reason,
		})

	} else {
		_, err = c.client.LeaveRoom(roomID)
	}

	if err != nil {
		c.logger.Error().Msg("Could not leave room due to: " + err.Error())
		return fmt.Errorf("could not leave room: %w", err)
	}

	node := c.GetOrCreateRoom(roomID)
	node.HasLeft = true
	node.Unload()
	fmt.Println("Left room with ID: " + roomID)
	c.logger.Info().Msg("Left room with ID: " + roomID.String())

	return nil
}

// After this function is called, a user will no longer be able to retrieve history for the given room.
// If all users on a homeserver forget a room, the room is eligible for deletion from that homeserver.
func (c *ClientWrapper) ForgetRoom(roomID id.RoomID) error {
	_, err := c.client.ForgetRoom(roomID)

	if err != nil {
		c.logger.Error().Err(err).Msg("could not forget room")
		return err
	}

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
		for i := 0; i < len(resp.JoinedRooms); i++ { //first indexes are the most recent rooms
			//fmt.Print(resp.JoinedRooms[i] + ", ")
			//if there aren't any rooms in memory, creates them
			c.config.Rooms.GetOrCreate(resp.JoinedRooms[i])

		}
	}

	return resp.JoinedRooms, err
}

// Joins a room with the given room or alias, through the specified server in the arguments
func (c *ClientWrapper) JoinRoom(roomID id.RoomID, server string) (*rooms.Room, error) {
	//maybe add a check to see if content is nil
	resp, err := c.client.JoinRoom(roomID.String(), server, nil)

	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not join room with the given id or alias")
		return nil, err
	}

	room := c.GetOrCreateRoom(resp.RoomID)
	room.HasLeft = false
	fmt.Println("Successfully joined room with id: " + roomID)
	c.logger.Info().Msg("Successfully joined room with ID: " + roomID.String())

	return room, nil
}

// Retrieves the list of members of the given room
func (c *ClientWrapper) JoinedMembers(roomID id.RoomID) ([]id.UserID, error) {
	resp, err := c.client.JoinedMembers(roomID)
	keys := make([]id.UserID, len(resp.Joined))

	if err != nil {
		fmt.Println(err)
		c.logger.Error().Err(err).Msg("could not get the list of members of the given room")
		return nil, err
	} else {
		i := 0
		for key := range resp.Joined { //print out the room members' display names
			fmt.Println(key)
			keys[i] = key
			i++
		}
	}

	return keys, nil
}

func (c *ClientWrapper) FetchMembers(room *rooms.Room) error {
	fmt.Print("Fetching member list for", room.ID)
	c.logger.Info().Msg("Fetching member list for room" + room.ID.String())
	members, err := c.client.Members(room.ID, mautrix.ReqMembers{At: room.LastPrevBatch})
	if err != nil {
		c.logger.Err(err).Msg("Could not fetch members of room " + room.ID.String())
		return err
	}
	c.logger.Info().Msg("Fetched " + strconv.Itoa(len(members.Chunk)) + "members for " + room.ID.String())
	for _, evt := range members.Chunk {
		err := evt.Content.ParseRaw(evt.Type)
		if err != nil {
			c.logger.Err(err).Msg("Failed to parse member event of " + evt.GetStateKey())
			continue
		}
		room.UpdateState(evt)
	}
	room.MembersFetched = true
	return nil
}

// GetHistory fetches room history.
func (c *ClientWrapper) GetHistory(room *rooms.Room, limit int, dbPointer uint64) ([]*mxevents.Event, uint64, error) {
	events, newDBPointer, err := c.history.Load(room, limit, dbPointer) //tries to obtain event history of the given room locally
	if err != nil {
		c.logger.Err(err).Msg("Could not load events of room " + room.ID.String() + " from local cache")
		return nil, dbPointer, err
	}

	if len(events) > 0 {
		fmt.Printf("Loaded %d events for %s from local cache", len(events), room.ID)
		c.logger.Info().Msg("Loaded " + strconv.Itoa(len(events)) + "from local cache of room with ID: " + room.ID.String())
		return events, newDBPointer, nil
	}

	resp, err := c.client.Messages(room.ID, room.PrevBatch, "", 'b', nil, limit) //otherwise fetch history from server
	if err != nil {
		c.logger.Err(err).Msg("Could not load events of room " + room.ID.String() + " from homeserver")
		return nil, dbPointer, err
	}

	fmt.Printf("Loaded %d events for %s from server", len(resp.Chunk), room.ID.String())
	c.logger.Info().Msg("Loaded " + string(rune(len(resp.Chunk))) + " events for " + room.ID.String() + " from server from " + resp.Start + " to " + resp.End)
	for i, evt := range resp.Chunk {
		err := evt.Content.ParseRaw(evt.Type)
		if err != nil {
			fmt.Printf("Failed to unmarshal content of event %s (type %s) by %s in %s: %v\n%s", evt.ID, evt.Type.Repr(), evt.Sender, evt.RoomID, err, string(evt.Content.VeryRaw))
			c.logger.Err(err).Msg("Failed to unmarshal content of event " + evt.ID.String() + " (type " + evt.Type.Repr() + ") by " + string(evt.Sender) + " in " + string(evt.RoomID) + " with content:" + string(evt.Content.VeryRaw))
		}

		if evt.Type == event.EventEncrypted {
			if c.crypto == nil {
				evt.Type = mxevents.EventEncryptionUnsupported
				origContent, _ := evt.Content.Parsed.(*event.EncryptedEventContent)
				evt.Content.Parsed = mxevents.EncryptionUnsupportedContent{Original: origContent}
			} else {
				decrypted, err := c.crypto.DecryptMegolmEvent(context.TODO(), evt)
				if err != nil {
					fmt.Printf("Failed to decrypt event %s: %v", evt.ID, err)
					c.logger.Err(err).Msg("Failed to decrypt event " + evt.ID.String())
					evt.Type = mxevents.EventBadEncrypted
					origContent, _ := evt.Content.Parsed.(*event.EncryptedEventContent)
					evt.Content.Parsed = &mxevents.BadEncryptedContent{
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
		return []*mxevents.Event{}, dbPointer, nil
	}
	// TODO newDBPointer isn't accurate in this case yet, fix later
	events, _, err = c.history.Prepend(room, resp.Chunk) //update event history
	if err != nil {
		return nil, dbPointer, err
	}
	return events, dbPointer, nil
}

// Fetches a specific event of the given room
func (c *ClientWrapper) GetEvent(room *rooms.Room, eventID id.EventID) (*mxevents.Event, error) {
	evt, err := c.history.Get(room, eventID) //First tries to obtain the event from the local cache
	if err != nil && err != ErrEventNotFound {
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

	c.logger.Info().Msg("Loaded event " + eventID.String() + " from server")
	return mxevents.Wrap(mxEvent), nil
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
func (c *ClientWrapper) SendEvent(evt *mxevents.Event) (id.EventID, error) {
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
func (c *ClientWrapper) SendStateEvent(evt *mxevents.Event) (id.EventID, error) {
	//TODO: Encryption flow before sending the event
	fmt.Println(evt.RoomID)
	fmt.Println(evt.Type)

	resp, err := c.client.SendStateEvent(evt.RoomID, evt.Type, *evt.StateKey, &evt.Content)
	if err != nil {
		c.logger.Error().Err(err).Msg("could not send the specified event")
		return "", err
	}

	c.logger.Info().Msg("Sent state event with event ID: " + resp.EventID.String())
	return resp.EventID, nil
}

// Sends a read receipt regarding the event in the arguments
func (c *ClientWrapper) SendReadReceipt(evt *mxevents.Event) error {
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

	c.addMessageToHistory(room, mxEvent)

	//Possivelmente fazer alguma coisa com o conteudo da mensagem (se for um alerta de intruso por exemplo)

}

func (c *ClientWrapper) HandleRoomEncryption(source mautrix.EventSource, mxEvent *event.Event) {
	roomID := mxEvent.RoomID
	room := c.GetOrCreateRoom(roomID)
	room.Encrypted = true
	c.logger.Info().Msg("Room with ID " + roomID.String() + " is now encrypted.")
}

func (c *ClientWrapper) HandleEncrypted(source mautrix.EventSource, mxEvent *event.Event) {
	evt, err := c.crypto.DecryptMegolmEvent(context.TODO(), mxEvent)
	if err != nil {
		c.logger.Error().Err(err).Msg("failed to decrypt event contents")
		mxEvent.Type = mxevents.EventBadEncrypted
		origContent, _ := mxEvent.Content.Parsed.(*event.EncryptedEventContent)
		mxEvent.Content.Parsed = &mxevents.BadEncryptedContent{
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

func (c *ClientWrapper) HandleEncryptedUnsupported(source mautrix.EventSource, mxEvent *event.Event) {
	mxEvent.Type = mxevents.EventEncryptionUnsupported
	origContent, _ := mxEvent.Content.Parsed.(*event.EncryptedEventContent)
	mxEvent.Content.Parsed = mxevents.EncryptionUnsupportedContent{Original: origContent}
	c.HandleMessage(source, mxEvent)
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

func (c *ClientWrapper) HandleReadReceipt(source mautrix.EventSource, evt *event.Event) {
	if source&mautrix.EventSourceLeave != 0 {
		return
	}

	room := c.GetRoom(evt.RoomID)
	lastReadEvent := c.parseReadReceipt(room, evt)
	if len(lastReadEvent) == 0 {
		return
	}

	if room != nil {
		room.MarkRead(lastReadEvent)
	}
}

func (c *ClientWrapper) parseReadReceipt(room *rooms.Room, evt *event.Event) (largestTimestampEvent id.EventID) {
	var largestTimestamp time.Time
	var offline offlineData

	var members, _ = c.JoinedMembers(evt.RoomID) //fetch from server

	//map[id.EventID]map[ReceiptType]map[id.UserID]ReadReceipt

	for eventID, receipts := range *evt.Content.AsReceipt() {
		myInfo, ok := receipts[event.ReceiptTypeRead][c.config.UserID]
		if !ok {
			continue
		}

		if myInfo.Timestamp.After(largestTimestamp) {
			largestTimestamp = myInfo.Timestamp
			largestTimestampEvent = eventID
		}

		//********** OFFLINE COMMS LOGIC ***********//

		actualEvent, _ := c.GetEvent(room, eventID)
		if actualEvent.Sender == c.client.UserID { //only send events that we sent
			ackUsers := receipts["m.read"] //get all users that saw the event with eventID
			//compare them against room members
			for _, user := range members {
				if _, ok := ackUsers[user]; !ok {
					offline.users = append(offline.users, user)
				}
			}

			//if at least one user did not send a receipt for this event
			if len(offline.users) > 0 {
				offline.eventID = eventID
				offline.roomID = evt.RoomID
				c.sendOff <- offline //send data to goroutine
			}
		} else {
			continue
		}

	}
	return
}

// Auxiliary method called whenever a message is received, whether the client is offline or online (through handleMessage)
func (c *ClientWrapper) addMessageToHistory(room *rooms.Room, mxEvent *event.Event) {
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

	//this will fail if the device is offline
	c.SendReadReceipt(evt) //Spec recommends not sending the receipt right as the message is received, but
	//in this case i think this is neglectable -> keep this in mind tho
	//Talvez so mandar o receipt quando o user usar o commando da history de uma sala?
}

func (c *ClientWrapper) FetchDeviceKeys(userToFetch id.UserID, deviceToFetch id.DeviceID) (id.Curve25519, id.Ed25519, error) {
	device := make(mautrix.DeviceIDList, 1)
	device = append(device, (id.DeviceID(deviceToFetch)))

	respKey, err := c.client.QueryKeys(&mautrix.ReqQueryKeys{ //turn into a function
		DeviceKeys: mautrix.DeviceKeysRequest{
			userToFetch: device,
		},
		Timeout: 10000,
	})

	if err != nil {
		c.logger.Err(err).Msg("Could not obtain user's device keys")
		return "", "", err
	}

	//The receiving user's keys, obtained from the server
	idKey := respKey.DeviceKeys[userToFetch][device[0]].Keys.GetCurve25519(device[0]) //identity key
	edKey := respKey.DeviceKeys[userToFetch][device[0]].Keys.GetEd25519(device[0])    //fingerprint key

	return idKey, edKey, nil
}

//****************** OFFLINE COMMS *********************//

const protocolID = "/matrix-offline/1.0.0"

// struct to hold data to send to clients outside of matrix if needed
type offlineData struct {
	eventID id.EventID  //the event to send
	roomID  id.RoomID   //the room to whom the event belongs to
	users   []id.UserID //the users that did not receive the event
}

func newHost() host.Host {
	// Set your own keypair
	//Would like to use matrix's Ed25519 fingerprint key pair, but the private part is never disclosed to the API
	priv, _, err := cryp.GenerateKeyPair(
		cryp.Ed25519, // Select your key type. Ed25519 are nice short
		-1,           // Select key length when possible (i.e. RSA).
	)
	if err != nil {
		panic(err)
	}

	//might need some tuning - want small groups
	connmgr, err := connmgr.NewConnManager(
		10, // Lowwater
		20, // HighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		panic(err)
	}
	host, err := libp2p.New(
		// Use the keypair we generated
		libp2p.Identity(priv),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/9000", // regular tcp connections
			//"/ip4/0.0.0.0/udp/9000/quic", // a UDP endpoint for the QUIC transport
		),
		// support TLS connections
		libp2p.Security(libp2ptls.ID, libp2ptls.New),
		// support any other default transports (TCP)
		libp2p.DefaultTransports,
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(connmgr),
		// Attempt to open ports using uPNP for NATed hosts.
		libp2p.NATPortMap(),
		// If you want to help other peers to figure out if they are behind
		// NATs, you can launch the server-side of AutoNAT too (AutoRelay
		// already runs the client)
		//
		// This service is highly rate-limited and should not cause any
		// performance issues.
		libp2p.EnableNATService(),
	)
	if err != nil {
		panic(err)
	}

	return host
}

func (c *ClientWrapper) runOffline() {

	// The context governs the lifetime of the libp2p node.
	// Cancelling it will stop the host.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host := newHost()

	defer host.Close()
	host.SetStreamHandler(protocolID, c.handleIncomingStream)

	peerChan := offline.InitMDNS(host, "matrix-offline")
	for {
		select {
		case <-c.sendOff: //if there is something to send, look for peers
			for { // allows multiple peers to join
				peer := <-peerChan // will block until we discover a peer
				fmt.Println("Found peer:", peer, ", connecting")
				c.logger.Info().Msg("Found peer: " + peer.String() + ", connecting")

				if err := host.Connect(ctx, peer); err != nil {
					fmt.Println("Connection failed:", err)
					c.logger.Err(err).Msg("Connection failed")
					continue
				}

				// open a stream, this stream will be handled by handleIncomingStream other end
				stream, err := host.NewStream(ctx, peer.ID, protocolID)

				if err != nil {
					fmt.Println("Stream open failed", err)
					c.logger.Err(err).Msg("Could not open stream")
				} else {
					rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

					fmt.Println("Connected to:", peer)
					c.logger.Info().Msg("Connected to: " + peer.String())
					go c.sendOffline(rw)

				}

				//stream.Close() //manter aberto
			}
		default:
			c.logger.Info().Msg("Listening for connections in case we are offline")
			select {} //thread hangs forever until the other case is true
		}
	}
}

func (c *ClientWrapper) handleIncomingStream(s network.Stream) {
	c.logger.Info().Msg("Got a new stream!")
	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	var wg sync.WaitGroup
	wg.Add(1)

	//if stream is incoming, it means we are offline, therefore only need to do the receiving end of the logic
	go func() {
		//Wrap the worker call in a closure that makes sure to tell the WaitGroup that this worker is done.
		//This way the worker itself does not have to be aware of the concurrency primitives involved in its execution.
		defer wg.Done()
		c.readData(rw)
	}()

	//go c.readData(rw)

	// stream 's' will stay open until you close it (or the other side closes it).
	wg.Wait() //wait for the readData subroutine to finish and close stream
	s.Close()
}

func (c *ClientWrapper) sendOffline(rw *bufio.ReadWriter) {
	toSend := <-c.sendOff
	room := c.GetRoom(toSend.roomID)
	evt, _ := c.GetEvent(room, toSend.eventID)

	offlineHost := c.exchangeCredentials(rw)

	if slices.Contains(toSend.users, offlineHost.UserID) {
		idKey, edKey, err := c.FetchDeviceKeys(offlineHost.UserID, id.DeviceID(offlineHost.DeviceID))
		if err != nil {
			return
		}

		//If there is an established Olm session with the identity key of the offline client, first assume a Megolm session has also been shared with the
		//offline device previously and send the encrypted event as normal. In case the offline client cannot decrypt it, then share the megolm session a posteriori.
		if b := c.crypto.CryptoStore.HasSession(idKey); b {
			encrypted, err := c.crypto.EncryptMegolmEvent(context.TODO(), evt.RoomID, evt.Type, &evt.Content)
			if err != nil {
				c.logger.Error().Err(err).Msg("Could not encrypt the specified event")
			}
			evt.Type = event.EventEncrypted
			evt.Content = event.Content{Parsed: encrypted}
			c.writeBytes(rw, evt)

			//Wait for the other client's response here -> can be a key request or an ACK (TODO: add a clause for this)
			keyReq := c.readBytes(rw)
			originalContent, _ := keyReq.Content.Parsed.(*event.RoomKeyRequestEventContent)
			forwardedRoomKey, _ := c.parseKeyRequestEvent(originalContent)

			olmSesh, _ := c.crypto.CryptoStore.GetLatestSession(idKey)

			olmContent := c.encryptOlm(idKey, edKey, olmSesh, offlineHost.UserID, event.ToDeviceForwardedRoomKey, forwardedRoomKey)
			olmEvent := &mxevents.Event{
				Event: &event.Event{
					Type:    event.ToDeviceEncrypted,
					Content: event.Content{Parsed: olmContent},
				},
			}
			c.writeBytes(rw, olmEvent)

		} //maybe add else clause here, even though it probably will never happen
	}
}

func (c *ClientWrapper) readData(rw *bufio.ReadWriter) {
	hostDevice := c.exchangeCredentials(rw)
	senderCredentials := map[id.UserID][]id.DeviceID{hostDevice.UserID: {hostDevice.DeviceID}}

	var missingEvt *mxevents.Event
	var evt *event.Event
	var err error

	missingEvt = c.readBytes(rw)
	room := c.GetOrCreateRoom(missingEvt.RoomID)

	if existing, _ := c.history.Get(room, evt.ID); existing != nil {
		c.logger.Info().Msg("Event is already stored, probably was already sent by another host")
		return
	}

	evt, err = c.crypto.DecryptMegolmEvent(context.TODO(), missingEvt.Event)

	if err != nil {
		c.logger.Err(err).Msg("Could not decrypt event received offline")

		//Ask for megolm session details here -> build m.room.key.request event
		keyReq := c.buildKeyRequest(evt.RoomID, evt.Content.AsEncrypted().SenderKey, evt.Content.AsEncrypted().SessionID, senderCredentials)
		c.writeBytes(rw, keyReq)

		//Receive encrypted megolm session
		encrypted := c.readBytes(rw)

		//Decrypt the event with Olm
		decryptedEvt, err := c.decryptOlm(context.Background(), encrypted.Event)
		if err != nil {
			c.logger.Err(err).Msg("Could not decrypt olm event")
		}
		decryptedContent := decryptedEvt.Content.Parsed.(*event.ForwardedRoomKeyEventContent)

		//Update the megolm session
		if c.importForwardedRoomKey(context.Background(), decryptedEvt, decryptedContent) {
			c.logger.Trace().Msg("Handled forwarded room key event")
			//Should now be able to decrypt
			evt, _ = c.crypto.DecryptMegolmEvent(context.TODO(), missingEvt.Event) //not expecting an err here
		}
	}

	c.addMessageToHistory(room, evt)
}

func (c *ClientWrapper) exchangeCredentials(rw *bufio.ReadWriter) *id.Device {
	//send our credentials to connected host
	selfID, err := c.crypto.CryptoStore.GetDevice(c.client.UserID, c.client.DeviceID)
	if err != nil { //since the store used by the crypto module is a db, this probably won't fail while offline
		c.logger.Err(err).Msg("Could not fetch own device info from crypto store")
	}

	marshalled, _ := json.Marshal(selfID)
	rw.Write(marshalled)

	//receive other host's client credentials
	var hostDevice *id.Device
	bytes, _ := rw.ReadBytes('\n')
	json.Unmarshal(bytes, hostDevice)

	//Should be safe to call both on and offline
	if trusted := c.crypto.IsDeviceTrusted(hostDevice); !trusted {
		c.logger.Info().Msg("Host device is not trusted")
		return nil
	}

	return hostDevice

}

func (c *ClientWrapper) writeBytes(rw *bufio.ReadWriter, evt *mxevents.Event) {
	reqBytes, err := json.Marshal(evt)
	if err != nil {
		c.logger.Err(err).Msg("Could not marshal key request event bytes")
	}

	if _, err := rw.Write(reqBytes); err != nil {
		c.logger.Error().Msg("Could not send bytes to stream")
	}
}

func (c *ClientWrapper) readBytes(rw *bufio.ReadWriter) *mxevents.Event {
	//Next, it will receive the encrypted event
	evtBytes, err := rw.ReadBytes('\n')
	if err != nil {
		// if it reaches this, need to find the correct delim
		c.logger.Error().Msg("Failed to read bytes from stream")
	}

	var evt *mxevents.Event
	json.Unmarshal(evtBytes, evt)

	return evt
}
