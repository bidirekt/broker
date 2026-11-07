package model_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestNewSessionMessage_KeepsASuccessKindAndText(t *testing.T) {
	assert.Equal(t,
		model.SessionMessage{Kind: model.MessageSuccess, Text: "participant created"},
		model.NewSessionMessage(model.MessageSuccess, "participant created"),
	)
}

func TestNewSessionMessage_PanicsOnAnUnknownKind(t *testing.T) {
	assert.Panics(t, func() { model.NewSessionMessage(model.MessageKind("warning"), "participant created") })
	assert.Panics(t, func() { model.NewSessionMessage(model.MessageKind(""), "participant created") })
}
