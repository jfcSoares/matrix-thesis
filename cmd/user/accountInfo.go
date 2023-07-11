/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package user

import (
	"fmt"

	"github.com/spf13/cobra"
)

// accountInfoCmd represents the accountInfo command
var accountInfoCmd = &cobra.Command{
	Use:   "accountInfo",
	Short: "Shows info about the account that is currently logged in.",
	Long: `Returns relevant information about the account that is currently logged
	in, including username, homeserver, device ID, and joined rooms.`,
	Run: func(cmd *cobra.Command, args []string) {
		userID := Backend.Config().GetUserID().String()
		hs := Backend.Config().Homeserver
		deviceID := Backend.Config().DeviceID.String()

		fmt.Println("Account username: " + userID)
		fmt.Println("Account server: " + hs)
		fmt.Println("Device ID: " + deviceID)
		var a []string
		for _, room := range Backend.Config().Rooms.Map {
			a = append(a, room.GetTitle())
		}
		fmt.Println(a)
	},
}

func init() {
	UserCmd.AddCommand(accountInfoCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// accountInfoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// accountInfoCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
