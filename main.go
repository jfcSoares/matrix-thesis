package main

import (
	matrix "matrix/matrix"
)

//var client *Client

func main() {
	c := matrix.NewContainer()
	c.InitClient(false, "")
	c.Login("test1", "Test1!´´´")

	//<-c.IsStopped()
	c.Logout()
}
