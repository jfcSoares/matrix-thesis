/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"fmt"
	"thesgo/matrix/mxevents"

	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var message string

// messageCmd represents the message command
var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Sends a message to the specified room.",
	Long: `Sends a message to the specified room, encrypted by default. 
	To see every message sent by every user in a room, use command "history".`,
	Run: func(cmd *cobra.Command, args []string) {
		evt := prepareEvent()
		Backend.Matrix().SendEvent(evt)
	},
}

// Builds the event to send to the server for processing
func prepareEvent() *mxevents.Event {
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    message,
	}

	//Maybe use user credentials stored in config file, rather than dynamically getting them from the matrix container
	evt := mxevents.Wrap(&event.Event{
		ID:       id.EventID(Backend.Matrix().Client().TxnID()),
		Sender:   Backend.Matrix().Client().UserID,
		Type:     event.EventMessage,
		RoomID:   id.RoomID(RoomName),
		Content:  event.Content{Parsed: content},
		Unsigned: event.Unsigned{TransactionID: Backend.Matrix().Client().TxnID()},
	})

	return evt
}

func init() {
	RoomCmd.AddCommand(messageCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// messageCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// messageCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	messageCmd.Flags().StringVarP(&message, "message", "m", "", "Message to send to the room")

	if err := messageCmd.MarkFlagRequired("message"); err != nil {
		fmt.Println(err)
	}
}
