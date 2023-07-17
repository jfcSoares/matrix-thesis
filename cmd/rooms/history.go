/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	"fmt"

	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Lists the most recent events in a room.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		room := Backend.Matrix().GetRoom(id.RoomID(RoomName))
		hist, _, _ := Backend.Matrix().GetHistory(room, 50, 0)
		for _, evt := range hist {
			fmt.Println(evt.Content.AsMessage().Body)
		}
	},
}

func init() {
	RoomCmd.AddCommand(historyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// historyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// historyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
