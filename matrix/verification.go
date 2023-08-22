package matrix

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// Struct to pass as the VerificationHook for the verification process
type VerificationContainer struct {
	device    *id.Device
	emojiText *EmojiView

	confirmChan chan bool
	done        bool
}

func NewVerificationContainer(device *id.Device, timeout time.Duration) *VerificationContainer {
	vc := &VerificationContainer{
		device:      device,
		done:        false,
		confirmChan: make(chan bool),
	}

	vc.emojiText = &EmojiView{}

	return vc
}

func (vc *VerificationContainer) VerificationMethods() []crypto.VerificationMethod {
	return []crypto.VerificationMethod{crypto.VerificationMethodEmoji{}, crypto.VerificationMethodDecimal{}}
}

func (vc *VerificationContainer) VerifySASMatch(otherDevice *id.Device, data crypto.SASData) bool {
	vc.device = otherDevice
	var typeName string
	if data.Type() == event.SASDecimal {
		typeName = "numbers"
	} else if data.Type() == event.SASEmoji {
		typeName = "emojis"
	} else {
		return false
	}

	fmt.Printf(
		"Check if the other device is showing the\n"+
			"same %s as below, then type \"yes\" to\n"+
			"accept, or \"no\" to reject", typeName)

	vc.emojiText.Data = data

	//Print emoji to console, wait for user input (Yes/No)
	vc.emojiText.Draw()
	go vc.awaitConfirm()

	confirm := <-vc.confirmChan
	vc.emojiText.Data = nil

	fmt.Printf("Waiting for %s\nto confirm", vc.device.UserID)

	return confirm

}

func (vc *VerificationContainer) OnCancel(cancelledByUs bool, reason string, _ event.VerificationCancelCode) {
	if cancelledByUs {
		fmt.Printf("Verification failed: %s", reason)
	} else {
		fmt.Printf("Verification cancelled by %s: %s", vc.device.UserID, reason)
	}

	//vm.inputBar.SetPlaceholder("Press enter to close the dialog")
	//vc.stopWaiting <- struct{}{}
	vc.done = true
}

func (vc *VerificationContainer) OnSuccess() {
	fmt.Printf("Successfully verified %s (%s) of %s", vc.device.Name, vc.device.DeviceID, vc.device.UserID)
	vc.done = true
}

func (vc *VerificationContainer) awaitConfirm() {
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.Replace(line, "\n", "", -1)
	//for windows: line = strings.Replace(line, "\r\n", "", -1)
	if strings.Compare(line, "yes") == 0 {
		//send signal to parent thread
		vc.confirmChan <- true
	} else {
		vc.confirmChan <- false
	}
}

type EmojiView struct {
	Data crypto.SASData
}

func (e *EmojiView) Draw() {
	if e.Data == nil {
		return
	}

	switch e.Data.Type() {
	case event.SASEmoji:
		for _, emoji := range e.Data.(crypto.EmojiSASData) {
			fmt.Print(emoji.Emoji)

		}
		fmt.Println() //Hacky way to have the description of each emoji below the respective emoji
		for _, emoji := range e.Data.(crypto.EmojiSASData) {
			fmt.Print(emoji.Description)

		}
	case event.SASDecimal:
		for _, number := range e.Data.(crypto.DecimalSASData) {
			fmt.Print(strconv.FormatUint(uint64(number), 10))
		}
	}

}
