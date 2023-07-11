/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package user

import (
	"github.com/spf13/cobra"
)

// logoutCmd represents the logout command
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logs the user account out of the client.",
	Long:  `Logs the user account out of the client, deleting their session by revoking the access token.`,
	Run: func(cmd *cobra.Command, args []string) {
		Backend.Matrix().Logout()
	},
}

func init() {
	UserCmd.AddCommand(logoutCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// logoutCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// logoutCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
