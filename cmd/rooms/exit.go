/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

var reason string

// exitCmd represents the exit command
var exitCmd = &cobra.Command{
	Use:   "exit",
	Short: "Exit an existing room.",
	Long: `Stops a user from participating in a given room, but it may still be able to retrieve its history
	if it rejoins the same room.`,
	Run: func(cmd *cobra.Command, args []string) {
		Backend.Matrix().ExitRoom(id.RoomID(RoomName), reason)
	},
}

func init() {
	RoomCmd.AddCommand(exitCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// exitCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// exitCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	exitCmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason to leave the room") //optional

}
