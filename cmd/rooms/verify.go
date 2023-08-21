/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package rooms

import (
	//"fmt"

	"fmt"
	"time"

	"thesgo/matrix"

	"github.com/spf13/cobra"
	"maunium.net/go/mautrix/id"
)

var userToVerify string

// verifyCmd represents the verify command
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify the device of another user in the room.",
	Long: `Performs in-room verification with another user in a room where both of them are present, through the 
	SAS verification method.`,
	Run: func(cmd *cobra.Command, args []string) {
		mach := Backend.Matrix().Crypto()
		device := &id.Device{UserID: id.UserID(userToVerify)}
		vc := matrix.NewVerificationContainer(device, mach.DefaultSASTimeout)
		_, err := mach.NewInRoomSASVerificationWith(id.RoomID(RoomName), id.UserID(userToVerify), vc, 120*time.Second)
		if err != nil {
			fmt.Printf("Failed to start in-room verification: %v", err)
			return
		}
	},
}

func init() {
	RoomCmd.AddCommand(verifyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// verifyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// verifyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	verifyCmd.Flags().StringVarP(&userToVerify, "user", "u", "", "User to verify within the room.")
	if err := verifyCmd.MarkPersistentFlagRequired("user"); err != nil {
		fmt.Println(err)
	}
}
