/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"fmt"

	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

var user string

// inviteCmd represents the invite command
var inviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite a user to an existing room.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		Backend.Matrix().InviteUser(id.RoomID(RoomName), reason, user)
	},
}

func init() {
	RoomCmd.AddCommand(inviteCmd)

	inviteCmd.Flags().StringVarP(&user, "username", "u", "", "User to invite")
	inviteCmd.Flags().StringVarP(&reason, "reason", "r", "", "Reason to invite user to the room") //optional

	if err := inviteCmd.MarkFlagRequired("username"); err != nil {
		fmt.Println(err)
	}
}
