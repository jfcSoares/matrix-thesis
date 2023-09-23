/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

var topic string
var inviteList []string

// newRoomCmd represents the newRoom command
var newRoomCmd = &cobra.Command{
	Use:   "newRoom",
	Short: "Creates a new room.",
	Long: `Creates a new room with the user as its owner, using
	the specified name and topic, and inviting every user specified in the invite list.`,
	Run: func(cmd *cobra.Command, args []string) {
		var invited []id.UserID
		for _, name := range inviteList {
			user := id.NewUserID(name, "https://lpgains.duckdns.org")
			invited = append(invited, user)
		}
		Backend.Matrix().NewRoom(RoomName, topic, invited)
	},
}

func init() {
	RoomCmd.AddCommand(newRoomCmd)

	//newRoomCmd.Flags().StringVarP(&roomName, "room-name", "n", "", "Name for the new room")
	newRoomCmd.Flags().StringVarP(&topic, "topic", "t", "", "Topic for the new room")                                            //optional
	newRoomCmd.Flags().StringArrayVarP(&inviteList, "invite list", "i", nil, "Any users you may want to invite to the new room") //Optional

}
