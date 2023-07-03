package main

import (
	matrix "thesgo/matrix"
	"thesgo/matrix/mxevents"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

//var client *Client

func main() {
	c := matrix.NewWrapper()
	c.InitClient(false)
	c.Login("test1", "Test1!´´´")

	//roomID, _ := c.NewRoom("Test Room", "For testing", nil)

	rooms, _ := c.RoomsJoined()
	c.JoinedMembers(rooms[0])

	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    "Hello World!",
	}

	evt := mxevents.Wrap(&event.Event{
		ID:       id.EventID(c.Client().TxnID()),
		Sender:   c.Client().UserID,
		Type:     event.EventMessage,
		RoomID:   rooms[0],
		Content:  event.Content{Parsed: content},
		Unsigned: event.Unsigned{TransactionID: c.Client().TxnID()},
	})

	c.SendEvent(evt)

	//<-c.IsStopped()
	c.Logout()
}
