/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"fmt"
	"strconv"
	"thesgo/matrix/mxevents"

	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var rotationPeriod int64
var rotationMessages int

// activateCmd represents the activate command
var activateCmd = &cobra.Command{
	Use:   "activate",
	Short: "Activates encryption in the given room",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		evt := prepEvent()

		_, err := Backend.Matrix().SendStateEvent(evt)
		if err == nil {
			fmt.Println("Room with ID " + id.RoomID(RoomName) + " is now encrypted.")
		} else {
			fmt.Printf("Could not activate encryption for room: %s", id.RoomID(RoomName))
		}
	},
}

func prepEvent() (evt *mxevents.Event) {
	fmt.Print(strconv.FormatInt(rotationPeriod, 10) + " - " + strconv.Itoa(rotationMessages))
	evt = &mxevents.Event{
		Event: &event.Event{
			Type:   event.StateEncryption,
			RoomID: id.RoomID(RoomName),
			Content: event.Content{Parsed: &event.EncryptionEventContent{
				Algorithm:              id.AlgorithmMegolmV1,
				RotationPeriodMillis:   rotationPeriod,
				RotationPeriodMessages: rotationMessages,
			}},
		},
	}

	return
}

func init() {
	RoomCmd.AddCommand(activateCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// activateCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// activateCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	activateCmd.Flags().Int64VarP(&rotationPeriod, "rotate-period", "t", 604800000, "How long, in milliseconds, the session should be used before changing it")
	activateCmd.Flags().IntVarP(&rotationMessages, "rotate-messages", "m", 100, "How many messages should be sent before changing the session")
}
