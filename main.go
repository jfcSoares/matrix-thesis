package main

import (
	matrix "matrix/matrix"
)

//var client *Client

func main() {
	c := matrix.NewWrapper()
	c.InitClient(false, "")
	c.Login("test1", "Test1!´´´")

	roomID, _ := c.NewRoom("Test Room", "For testing", nil)

	c.JoinedMembers(roomID)
	c.RoomsJoined()

	//<-c.IsStopped()
	c.Logout()
}
