package events

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
