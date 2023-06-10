package main

import (
	"fmt"

	"maunium.net/go/mautrix"
)

type Client struct {
	client *mautrix.Client
}

/*type mxLogger struct{}

/func (log mxLogger) Debugfln(message string, args ...interface{}) {
	debug.Printf("[Matrix] "+message, args...)
}*/

/*var (
	NoHomeserver   = errors.New("no homeserver entered")
	ServerOutdated = errors.New("homeserver is outdated")
)*/

// NewClient creates a new Client for the given client instance.
func NewClient() *Client {
	var cli *mautrix.Client
	cli, _ = mautrix.NewClient("https://lpgains.duckdns.org", "", "")
	c := &Client{
		client: cli,
	}

	c.initClient()

	return c
}

// initializes the client and connects to the homeserver specified in the config.
func (c *Client) initClient() error {

	//c.client.UserAgent = fmt.Sprintf("gomuks/%s %s", c.gmx.Version(), mautrix.DefaultUserAgent)
	//c.client.Logger = mxLogger{}
	//c.client.DeviceID = c.config.DeviceID
	return nil
}

// Login sends a password login request with the given username and password.
func (c *Client) Login(user, password string) error {
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
func (c *Client) PasswordLogin(user, password string) error {

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

	c.ConcludeLogin(resp)

	return nil
}

// Concludes the login process, by assigning some last values to config fields
func (c *Client) ConcludeLogin(resp *mautrix.RespLogin) {
	fmt.Println(resp.UserID.String() + c.client.UserID.String())
	fmt.Println(resp.AccessToken + c.client.AccessToken)
	//go c.Start()
}

func (c *Client) Logout() {
	c.client.Logout()
	c.client.ClearCredentials()
}
