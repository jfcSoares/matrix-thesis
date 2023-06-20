package matrix

import (
	"errors"
	"fmt"
	"matrix/matrix/events"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type ClientWrapper struct {
	client  *mautrix.Client
	syncer  *mautrix.DefaultSyncer
	running bool
	stop    chan bool
}

/*type mxLogger struct{}

/func (log mxLogger) Debugfln(message string, args ...interface{}) {
	debug.Printf("[Matrix] "+message, args...)
}*/

var MinSpecVersion = mautrix.SpecV11
var SkipVersionCheck = false

var (
	ErrNoHomeserver   = errors.New("no homeserver entered")
	ErrServerOutdated = errors.New("homeserver is outdated")
)

// NewContainer creates a new Container for the given client instance.
func NewWrapper() *ClientWrapper {
	c := &ClientWrapper{
		running: false,
	}

	return c
}

// initializes the client and connects to the specified homeserver
func (c *ClientWrapper) InitClient(isStartup bool, userID id.UserID) error {

	//c.client.UserAgent = fmt.Sprintf("gomuks/%s %s", c.gmx.Version(), mautrix.DefaultUserAgent)
	//c.client.Logger = mxLogger{}
	//c.client.DeviceID = c.config.DeviceID

	if c.Initialized() {
		c.Stop()
		c.client = nil
	}

	/*var mxid id.UserID
	var accessToken string
	if len(c.client.AccessToken) > 0 { //if a a client's credentials are still saved
		accessToken = c.client.AccessToken
		mxid = c.client.UserID
	} else {
		mxid = userID
	}*/

	var err error
	c.client, err = mautrix.NewClient("https://lpgains.duckdns.org", "", "")
	if err != nil {
		return fmt.Errorf("failed to create mautrix client: %w", err)
	}

	/*allowInsecure := len(os.Getenv("CLIENT_ALLOW_INSECURE_CONNECTIONS")) > 0
	if allowInsecure {
		c.client.Client = &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		}
	}*/

	if !SkipVersionCheck && (!isStartup || len(c.client.AccessToken) > 0) {
		fmt.Printf("Checking versions that %s supports.", c.client.HomeserverURL)
		resp, err := c.client.Versions()
		if err != nil {
			fmt.Print("Error checking supported versions:", err)
			return fmt.Errorf("failed to check server versions: %w", err)
		} else if !resp.ContainsGreaterOrEqual(MinSpecVersion) {
			fmt.Print("Server doesn't support modern spec versions.")
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
			fmt.Print("Server supports modern spec versions")
		}
	}

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
		return err
	}

	for _, flow := range resp.Flows {
		if flow.Type == "m.login.password" {
			return c.PasswordLogin(user, password)
		} else if flow.Type == "m.login.sso" {
			return fmt.Errorf("SSO login method is not supported")
		} else {
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
		return err
	}

	c.concludeLogin(resp)

	return nil
}

// Concludes the login process, by assigning some last values to config fields
func (c *ClientWrapper) concludeLogin(resp *mautrix.RespLogin) {
	fmt.Println(resp.UserID.String() + " = " + c.client.UserID.String())
	fmt.Println(resp.AccessToken + c.client.AccessToken)

	//go c.Start()
}

func (c *ClientWrapper) Logout() {
	fmt.Println("Logging out...")
	c.client.Logout()
	c.Stop()
	c.client.ClearCredentials()
	c.client = nil
}

// Obtains information about the device currently being used
func (c *ClientWrapper) DeviceInfo() {
	resp, err := c.client.GetDeviceInfo(c.client.DeviceID)
	if err != nil {
		fmt.Println("Failed to obtain device info: %w", err)
	}
	fmt.Println(resp.DeviceID.String())
	fmt.Println(resp.DisplayName)
	fmt.Println(resp.LastSeenIP)
	fmt.Println(resp.LastSeenTS)
}

func (c *ClientWrapper) Synchronize() {
	c.client.Sync()
}

func (c *ClientWrapper) Start() {
	if c.client == nil {
		return
	}

	fmt.Print("Starting sync...")
	c.running = true
	c.client.StreamSyncMinAge = 30 * time.Minute
	for {
		select {
		case <-c.stop:
			fmt.Print("Stopping sync...")
			c.running = false
			return
		default:
			if err := c.client.Sync(); err != nil {
				if errors.Is(err, mautrix.MUnknownToken) {
					fmt.Print("Sync() errored with ", err, " -> logging out")
					// TODO support soft logout
					c.Logout()
				} else {
					fmt.Print("Sync() errored", err)
				}
			} else {
				fmt.Print("Sync() returned without error")
				c.Logout() //ONLY FOR TESTING
			}
		}
	}
}

// Stop stops the Matrix syncer.
func (c *ClientWrapper) Stop() {
	if c.running {
		fmt.Print("Stopping Matrix client...")
		select {
		case c.stop <- true:
		default:
		}
		c.client.StopSync()
		/*fmt.Print("Closing history manager...")
		err := c.history.Close()
		if err != nil {
			debug.Print("Error closing history manager:", err)
		}
		c.history = nil
		if c.crypto != nil {
			debug.Print("Flushing crypto store")
			err = c.crypto.FlushStore()
			if err != nil {
				debug.Print("Error flushing crypto store:", err)
			}
		}*/
	}
}

// OnLogin initializes the syncer and updates the room list.
/*func (c *ClientWrapper) OnLogin() {
	//c.cryptoOnLogin()
	//c.ui.OnLogin()


	debug.Print("Initializing syncer")
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
	//TODO: Add custom event handler for bluetooth comms maybe?
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
func (c *ClientWrapper) NewRoom(roomName string, topic string, inviteList []id.UserID) (id.RoomID, error) {
	resp, err := c.client.CreateRoom(&mautrix.ReqCreateRoom{
		Preset: "trusted_private_chat",
		Name:   roomName,
		Topic:  topic,
		Invite: inviteList,
	})
	fmt.Println("Created room with ID: " + resp.RoomID)
	c.client.Log.Info().Msg("Created room with ID: " + resp.RoomID.String())

	if err != nil {
		return "", err
	}

	return resp.RoomID, nil
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
		c.client.Log.Err(err).Msg("Could not leave room")
		return fmt.Errorf("could not leave room: %w", err)
	} else {
		fmt.Println(resp)
		fmt.Println("Left room with ID: " + roomID)
		c.client.Log.Info().Msg("Left room with ID: " + roomID.String())
	}

	return nil
}

// After this function is called, a user will no longer be able to retrieve history for the given room.
// If all users on a homeserver forget a room, the room is eligible for deletion from that homeserver.
func (c *ClientWrapper) ForgetRoom(roomID id.RoomID) error {
	resp, err := c.client.ForgetRoom(roomID)

	if err != nil {
		return err
	}

	fmt.Println(resp)
	fmt.Println("Forgot room with ID: " + roomID)
	c.client.Log.Info().Msg("Forgot the room with id: " + roomID.String())
	return nil
}

// Invites the given user to the room corresponding to the provided roomID
func (c *ClientWrapper) InviteUser(roomID id.RoomID, reason, user string) error {
	resp, err := c.client.InviteUser(roomID, &mautrix.ReqInviteUser{
		Reason: reason,
		UserID: id.UserID(user),
	})

	if err != nil {
		fmt.Println(err)
		return err
	} else {
		fmt.Println(resp)
		fmt.Println("Successfully Invited " + user + " to the room with id: " + roomID.String())
		c.client.Log.Info().Msg("Successfully invited " + user + " to the room with id: " + roomID.String())
	}

	return nil
}

// Lists the rooms the user is currently joined into
func (c *ClientWrapper) RoomsJoined() error {
	resp, err := c.client.JoinedRooms()

	if err != nil {
		fmt.Println(err)
		return err
	} else {
		for i := 0; i < len(resp.JoinedRooms); i++ {
			fmt.Println(resp.JoinedRooms[i])
		}
	}

	return nil
}

// Joins a room with the given room or alias, through the specified server in the arguments
func (c *ClientWrapper) JoinRoom(roomIdOrAlias, server string, content interface{}) error {
	//maybe add a check to see if content is nil
	resp, err := c.client.JoinRoom(roomIdOrAlias, server, content)

	if err != nil {
		fmt.Println(err)
		return err
	} else {
		fmt.Println(resp)
	}

	return nil
}

// Joins a room with the given roomID
func (c *ClientWrapper) JoinRoomByID(roomID id.RoomID) error {
	resp, err := c.client.JoinRoomByID(roomID)

	if err != nil {
		fmt.Println(err)
		return err
	} else {
		fmt.Println(resp)
		fmt.Println("Joined room with ID: " + roomID)
	}

	return nil
}

// Retrieves the list of members of the given room
func (c *ClientWrapper) JoinedMembers(roomID id.RoomID) error {
	resp, err := c.client.JoinedMembers(roomID)

	if err != nil {
		fmt.Println(err)
		return err
	} else {
		for key := range resp.Joined { //print out the room members' display names
			fmt.Println(key)
		}
	}

	return nil
}

//*************************** EVENTS *******************************//

// Sends a message event into a room
func (c *ClientWrapper) SendMessageEvent(evt *events.Event) (id.EventID, error) {
	//TODO: Encryption flow before sending the event

	resp, err := c.client.SendMessageEvent(evt.RoomID, evt.Type, &evt.Content, mautrix.ReqSendEvent{TransactionID: evt.Unsigned.TransactionID})
	if err != nil {
		return "", err
	}
	return resp.EventID, nil
}

// Sends a state event into a room
func (c *ClientWrapper) SendStateEvent(evt *events.Event) (id.EventID, error) {
	//TODO: Encryption flow before sending the event

	resp, err := c.client.SendStateEvent(evt.RoomID, evt.Type, *evt.StateKey, &evt.Content)
	if err != nil {
		return "", err
	}
	return resp.EventID, nil
}

// Sends a read receipt regarding the event in the arguments
func (c *ClientWrapper) SendReadReceipt(evt *events.Event) error {
	err := c.client.SendReceipt(evt.RoomID, evt.ID, event.ReceiptTypeRead, nil)

	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

// Get the state events for the current state of the given room
func (c *ClientWrapper) GetState(roomID id.RoomID) (*mautrix.RoomStateMap, error) {
	resp, err := c.client.State(roomID)

	if err != nil {
		return nil, err
	}

	return &resp, nil
}

/*func (c *ClientWrapper) SendToDevice(eventType event.Type, req *mautrix.ReqSendToDevice) (*mautrix.RespSendToDevice, error) {
	probably will not need this
}*/
