/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

// joinCmd represents the join command
var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Joins an existing room.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		Backend.Matrix().JoinRoom(id.RoomID(RoomName), Server)
	},
}

func init() {
	RoomCmd.AddCommand(joinCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// joinCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// joinCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
