package contract_test

import (
	"testing"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/stretchr/testify/assert"
)

func TestResourcePath_IsConsumer_DecidesByFirstSegment(t *testing.T) {
	path := contract.NewResourcePath("consumes;provides;rest;/x;get;responses;200")

	assert.True(t, path.IsConsumer())
	assert.False(t, path.IsProvider())
}

func TestResourcePath_IsProvider_DecidesByFirstSegment(t *testing.T) {
	path := contract.NewResourcePath("provides;rest;/consumes;get;responses;200")

	assert.True(t, path.IsProvider())
	assert.False(t, path.IsConsumer())
}
