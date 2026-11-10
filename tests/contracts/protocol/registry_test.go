package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
)

type namedProtocol struct {
	name model.Protocol
}

func (this namedProtocol) Protocol() model.Protocol {
	return this.name
}

func (this namedProtocol) Grammar() descriptor.Node {
	return descriptor.Map(descriptor.Key{}, descriptor.String())
}

func (this namedProtocol) ToResourceDeclarations(protocol.Block) []protocol.ResourceDeclaration {
	return nil
}

func protocolNames(registry *protocol.Registry) []model.Protocol {
	var names []model.Protocol
	for _, registered := range registry.Protocols() {
		names = append(names, registered.Protocol())
	}

	return names
}

func TestNewRegistry_SortsTheProtocolsByName(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{namedProtocol{"rest"}, namedProtocol{"graphql"}, namedProtocol{"grpc"}})

	assert.Equal(t, []model.Protocol{"graphql", "grpc", "rest"}, protocolNames(registry))
}

func TestNewRegistry_AProtocolRegisteredTwice_Panics(t *testing.T) {
	assert.PanicsWithError(t, `protocol "rest" is registered twice`, func() {
		protocol.NewRegistry([]protocol.Protocol{namedProtocol{"rest"}, namedProtocol{"rest"}})
	})
}

func TestGrammar_PlacesEachProtocolBlockUnderProvidesAndUnderEachConsumedService(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{restcontracts.New(), namedProtocol{"graphql"}})
	document := map[string]any{
		"provides": map[string]any{
			"rest":    map[string]any{"/pets": map[string]any{"get": map[string]any{"responses": map[string]any{"999": "Pet"}}}},
			"graphql": map[string]any{"pets": 1},
		},
		"consumes": map[string]any{
			"pets_api": map[string]any{"graphql": map[string]any{"pets": true}},
		},
	}

	violations := descriptor.Validate(registry.Grammar(), document, "api.yaml")

	require.Len(t, violations, 3)
	assert.Equal(t, "consumes;pets_api;graphql;pets", violations[0].Path)
	assert.Equal(t, "provides;graphql;pets", violations[1].Path)
	assert.Equal(t, "status.out_of_range", violations[2].ErrorCode)
	assert.Equal(t, "provides;rest;/pets;get;responses;999", violations[2].Path)
}

func TestGrammar_AnUnregisteredProtocolBlockIsAnUnknownField(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{restcontracts.New()})

	violations := descriptor.Validate(registry.Grammar(), map[string]any{"provides": map[string]any{"grpc": map[string]any{}}}, "api.yaml")

	require.Len(t, violations, 1)
	assert.Equal(t, "provides;grpc", violations[0].Path)
}
