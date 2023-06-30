// As per the terms described in the GNU Affero General Public License, published by
// the Free Software Foundation, which apply to the contents of the repository
// (https://github.com/tulir/gomuks), the code present in this file was based off of
// the content.go file in that repository

package mxevents

import (
	"encoding/gob"
	"reflect"

	"maunium.net/go/mautrix/event"
)

// create two new types of events: when a message event is badly encrypted; another for when a room does not support encryption
var EventBadEncrypted = event.Type{Type: "net.maunium.gomuks.bad_encrypted", Class: event.MessageEventType}
var EventEncryptionUnsupported = event.Type{Type: "net.maunium.gomuks.encryption_unsupported", Class: event.MessageEventType}

type BadEncryptedContent struct {
	Original *event.EncryptedEventContent `json:"-"`

	Reason string `json:"-"` //motive for the event encryption being rejected
}

type EncryptionUnsupportedContent struct {
	Original *event.EncryptedEventContent `json:"-"` //the original event content
}

// register the new event types to the local database, mapping the event type to its content type
func init() {
	gob.Register(&BadEncryptedContent{})
	gob.Register(&EncryptionUnsupportedContent{})
	event.TypeMap[EventBadEncrypted] = reflect.TypeOf(&BadEncryptedContent{})
	event.TypeMap[EventEncryptionUnsupported] = reflect.TypeOf(&EncryptionUnsupportedContent{})
}
