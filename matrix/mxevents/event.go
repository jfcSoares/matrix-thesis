// As per the terms described in the GNU Affero General Public License, published by
// the Free Software Foundation, which apply to the contents of the repository
// (https://github.com/tulir/gomuks), the code present in this file was based off of
// the event.go file in that repository
package mxevents

import (
	"maunium.net/go/mautrix/event"
)

type Event struct {
	*event.Event
	Cont EventContent `json:"-"`
}

func (evt *Event) SomewhatDangerousCopy() *Event {
	base := *evt.Event
	content := *base.Content.Parsed.(*event.MessageEventContent)
	evt.Content.Parsed = &content
	return &Event{
		Event: &base,
		Cont:  evt.Cont,
	}
}

func Wrap(event *event.Event) *Event {
	return &Event{Event: event}
}

type OutgoingState int

const (
	StateDefault OutgoingState = iota
	StateLocalEcho
	StateSendFail
)

type EventContent struct {
	OutgoingState OutgoingState
	Edits         []*Event
}
