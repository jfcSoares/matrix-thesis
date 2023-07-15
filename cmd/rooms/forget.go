/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

// forgetCmd represents the forget command
var forgetCmd = &cobra.Command{
	Use:   "forget",
	Short: "Forget an existing room.",
	Long: `When a user forgets a room, it will no longer be able to retrieve history for the given room, and
	iff all users on a homeserver forget a room, the room is eligible for deletion from that homeserver.`,
	Run: func(cmd *cobra.Command, args []string) {
		Backend.Matrix().ForgetRoom(id.RoomID(RoomName))
	},
}

func init() {
	RoomCmd.AddCommand(forgetCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// forgetCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// forgetCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
