package model

import "fmt"

type MessageKind string

const MessageSuccess MessageKind = "success"

type SessionMessage struct {
	Kind MessageKind `json:"kind"`
	Text string      `json:"text"`
}

func NewSessionMessage(kind MessageKind, text string) SessionMessage {
	if kind != MessageSuccess {
		panic(fmt.Errorf("error creating session message: unknown kind %q", kind))
	}

	return SessionMessage{Kind: kind, Text: text}
}
