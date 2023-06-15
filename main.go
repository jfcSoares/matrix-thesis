package main

import (
	matrix "matrix/matrix"
)

//var client *Client

func main() {
	c := matrix.NewContainer()
	c.InitClient(false, "")
	c.Login("test1", "Test1!´´´")

	c.NewRoom("Test Room", "For testing", nil)

	//<-c.IsStopped()
	c.Logout()
}
