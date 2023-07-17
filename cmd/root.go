/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"thesgo/cmd/rooms"
	"thesgo/cmd/user"
	ifc "thesgo/interfaces"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "thesgo",
	Short: "A PoC Matrix Client for a master's thesis",
	Long: `A Matrix client with the minimum functionalities provided by the Matrix API, with the inclusion of 
	E2E encryption, and additional offline communication between clients for a future context of an IoT system
	with multiple devices.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func addSubcommandGroups() {
	rootCmd.AddCommand(user.UserCmd) //adds the user commands as a whole subgroup
	rootCmd.AddCommand(rooms.RoomCmd)
}

// Set a variable in each command package pointing to the main client object (ifc.Thesgo)
func SetLinkToBackend(thesgo ifc.Thesgo) {
	user.SetLinkToBackend(thesgo)
	rooms.SetLinkToBackend(thesgo)
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.thesgo.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	addSubcommandGroups()
}
