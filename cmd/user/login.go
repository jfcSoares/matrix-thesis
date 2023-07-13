/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package user

import (
	"fmt"

	"github.com/spf13/cobra"
)

var userID, password string //variables to hold flag values

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:     "login",
	Short:   "Logs a user into his Matrix account",
	Long:    `Logs a user into his Matrix account, persisting his account and session information to a given file`,
	Example: "thesgo user login -u 'id' -p 'password'",
	Run: func(comd *cobra.Command, args []string) {
		Backend.Matrix().Login(userID, password)

	},
}

func init() {
	loginCmd.Flags().StringVarP(&userID, "username", "u", "", "Account username to login")
	loginCmd.Flags().StringVarP(&password, "password", "p", "", "Account password to login")

	if err := loginCmd.MarkFlagRequired("username"); err != nil {
		fmt.Println(err)
	}

	if err := loginCmd.MarkFlagRequired("password"); err != nil {
		fmt.Println(err)
	}

	UserCmd.AddCommand(loginCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// loginCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// loginCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
