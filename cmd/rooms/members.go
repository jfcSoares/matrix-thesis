/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

// membersCmd represents the members command
var membersCmd = &cobra.Command{
	Use:   "members",
	Short: "Fetches member list for the given room.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		Backend.Matrix().JoinedMembers(id.RoomID(RoomName))
		// for now, go with JoinedMembers //TODO: look into FetchMembers
	},
}

func init() {
	RoomCmd.AddCommand(membersCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// membersCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// membersCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
