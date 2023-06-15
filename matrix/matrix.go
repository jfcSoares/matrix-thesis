package matrix

import (
	"errors"
	"fmt"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

type Container struct {
	client  *mautrix.Client
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
func NewContainer() *Container {
	c := &Container{
		running: false,
	}

	return c
}

// initializes the client and connects to the specified homeserver
func (c *Container) InitClient(isStartup bool, userID id.UserID) error {

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
func (c *Container) Client() *mautrix.Client {
	return c.client
}

func (c *Container) IsStopped() chan bool {
	return c.stop
}

// Initialized returns whether or not the matrix client is initialized, i.e., has been instantiated
func (c *Container) Initialized() bool {
	return c.client != nil
}

// Login sends a password login request with the given username and password.
func (c *Container) Login(user, password string) error {
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
func (c *Container) PasswordLogin(user, password string) error {

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
func (c *Container) concludeLogin(resp *mautrix.RespLogin) {
	fmt.Println(resp.UserID.String() + " = " + c.client.UserID.String())
	fmt.Println(resp.AccessToken + c.client.AccessToken)

	//go c.Start()
}

func (c *Container) Logout() {
	fmt.Println("Logging out...")
	c.client.Logout()
	c.Stop()
	c.client.ClearCredentials()
	c.client = nil
}

func (c *Container) DeviceInfo() {
	resp, err := c.client.GetDeviceInfo(c.client.DeviceID)
	if err != nil {
		fmt.Println("Failed to obtain device info: %w", err)
	}
	fmt.Println(resp.DeviceID.String())
	fmt.Println(resp.DisplayName)
	fmt.Println(resp.LastSeenIP)
	fmt.Println(resp.LastSeenTS)
}

func (c *Container) Synchronize() {
	c.client.Sync()
}

func (c *Container) Start() {
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
func (c *Container) Stop() {
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

/*************************** ROOMS *******************************/

func (c *Container) NewRoom(roomName string, topic string, inviteList []id.UserID) error {
	resp, err := c.client.CreateRoom(&mautrix.ReqCreateRoom{
		Preset: "trusted_private_chat",
		Name:   roomName,
		Topic:  topic,
		Invite: inviteList,
	})
	fmt.Println("Room:", resp.RoomID)

	if err != nil {
		return err
	}

	return nil
}

// Stops a user from participating in a given room, but it may still be able to retrieve its history
// if it rejoins the same room
func (c *Container) ExitRoom(roomID id.RoomID, reason string) error {
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
		return fmt.Errorf("could not leave room: %w", err)
	} else {
		fmt.Println(resp)
	}

	return nil
}

// After this function is called, a user will no longer be able to retrieve history for the given room.
// If all users on a homeserver forget a room, the room is eligible for deletion from that homeserver.
func (c *Container) ForgetRoom(roomID id.RoomID) error {
	resp, err := c.client.ForgetRoom(roomID)

	if err != nil {
		return err
	}

	fmt.Println(resp)
	return nil
}
