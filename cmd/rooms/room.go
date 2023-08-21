/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"fmt"

	ifc "thesgo/interfaces"

	"github.com/spf13/cobra"
)

var Backend ifc.Thesgo                             //variable to handle client operations
var RoomName string                                //variable to hold roomID in all commands pertaining to rooms
const Server string = "https:/lpgains.duckdns.org" //const to avoid hardcoding server name

// roomCmd represents the room command
var RoomCmd = &cobra.Command{
	Use:   "room",
	Short: "Commands for every expected action regarding rooms.",
	Long: `Commands for a user that is logged in to interact with his rooms, such as inviting other users,
creating a new room, leaving a room, sending messages into a room (with encryption enabled by default) and
verifying other uses in the room.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("room called")
	},
}

// Set a variable pointing to the main client object (ifc.Thesgo)
func SetLinkToBackend(thesgo ifc.Thesgo) {
	Backend = thesgo
}

func init() {

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// roomCmd.PersistentFlags().String("foo", "", "A help for foo")
	//RoomCmd.PersistentFlags().BoolVarP("encryption")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// roomCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	RoomCmd.PersistentFlags().StringVarP(&RoomName, "room-name", "n", "", "Name of the room")
	if err := RoomCmd.MarkPersistentFlagRequired("room-name"); err != nil {
		fmt.Println(err)
	}

}
