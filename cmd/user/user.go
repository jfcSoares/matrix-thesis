/*
Copyright © 2023 João Soares <jfc.soares@campus.fct.unl.pt>
*/
package user

import (
	ifc "thesgo/interfaces"

	"github.com/spf13/cobra"
)

var Backend ifc.Thesgo //variable to handle client operations

// userCmd represents the user command
var UserCmd = &cobra.Command{
	Use:   "user",
	Short: "User is a command group for user-related commands",
	Long:  `Command group for user-related commands, such as login, logout or account-info`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
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
	// userCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// userCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
