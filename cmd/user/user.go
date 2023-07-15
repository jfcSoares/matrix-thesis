/*
Copyright © 2023 João Soares <jfc.soares@campus.fct.unl.pt>
*/
package user

import (
	"fmt"
	"os"
	ifc "thesgo/interfaces"

	"github.com/spf13/cobra"
)

var Backend ifc.Thesgo //variable to handle client operations
var cleancache, cleandata bool

const Server string = "https:/lpgains.duckdns.org"

// userCmd represents the user command
var UserCmd = &cobra.Command{
	Use:   "user",
	Short: "User is a command group for user-related commands",
	Long:  `Command group for user-related commands, such as login, logout or account-info`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		if cleancache {
			Backend.Config().Clear()
			fmt.Printf("Cleared cache at %s\n", Backend.Config().CacheDir)
		}

		if cleandata {
			Backend.Config().Clear()
			Backend.Config().ClearData()
			_ = os.RemoveAll(Backend.Config().Dir)
			fmt.Printf("Cleared cache at %s, data at %s and config at %s\n", Backend.Config().CacheDir, Backend.Config().DataDir, Backend.Config().Dir)
		}
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
	//userCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	UserCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	UserCmd.PersistentFlags().BoolVarP(&cleancache, "clear-cache", "c", false, "Instructs the client to clear the cache contents")
	UserCmd.PersistentFlags().BoolVarP(&cleandata, "clear-data", "d", false, "Instructs the client to clear all data previously stored")
}
