package main

//var client *Client

func NewInstance() *Client {
	test := NewClient()

	return test
}

func main() {
	cli := NewInstance()
	cli.Login("test1", "Test1!´´´")
	cli.Logout()
}
