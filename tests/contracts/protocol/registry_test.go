package protocol_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

type namedProtocol struct {
	name model.Protocol
}

func (this namedProtocol) Protocol() model.Protocol {
	return this.name
}

func (this namedProtocol) Grammar() protocol.Grammar {
	block := descriptor.Map(descriptor.Key{}, descriptor.String())

	return protocol.Grammar{Provides: block, Consumes: block}
}

func (this namedProtocol) ToDeclarations(protocol.Block) protocol.BlockDeclarations {
	return protocol.BlockDeclarations{}
}

type scalarProtocol struct {
	namedProtocol
}

func (this scalarProtocol) SchemaVocabulary() descriptor.Vocabulary {
	return descriptor.Vocabulary{Types: []descriptor.TypeVocabulary{{
		Name: "scalar",
		Keys: descriptor.Keys{"name": func(descriptor.Vocabulary) descriptor.Node { return descriptor.String() }},
	}}}
}

type labeledUnionProtocol struct {
	namedProtocol
}

func (this labeledUnionProtocol) SchemaVocabulary() descriptor.Vocabulary {
	return descriptor.Vocabulary{Types: []descriptor.TypeVocabulary{{
		Name: model.UnionType,
		Keys: descriptor.Keys{"label": func(descriptor.Vocabulary) descriptor.Node { return descriptor.String() }},
	}}}
}

type ruledProtocol struct {
	namedProtocol
	codes []string
}

func (this ruledProtocol) Rules() []protocol.Rule {
	var rules []protocol.Rule
	for _, code := range this.codes {
		rules = append(rules, func(protocol.Declarations) []violation.Violation {
			return []violation.Violation{{ErrorCode: code}}
		})
	}

	return rules
}

type joiningProtocol struct {
	namedProtocol
}

func (this joiningProtocol) MergeExtensions(pieces []protocol.ExtensionPiece) (any, []violation.Violation) {
	var sources []string
	for _, piece := range pieces {
		sources = append(sources, piece.Source+"="+piece.Value.(string))
	}

	return strings.Join(sources, ","), []violation.Violation{{ErrorCode: "extension.joined", Source: pieces[0].Source}}
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

func TestGrammar_EachSideTakesTheProtocolGrammarOfItsSide(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{sidedProtocol{}})
	document := map[string]any{
		"provides": map[string]any{"sided": map[string]any{"graph": "pets", "query": "pet"}},
		"consumes": map[string]any{"pets_api": map[string]any{"sided": map[string]any{"graph": "pets", "query": "pet"}}},
	}

	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "key.unknown",
			Path:      "consumes;pets_api;sided;graph",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "graph"},
		},
		{
			ErrorCode: "key.unknown",
			Path:      "provides;sided;query",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "query"},
		},
	}, descriptor.Validate(registry.Grammar(), document, "api.yaml"))
}

type sidedProtocol struct{}

func (this sidedProtocol) Protocol() model.Protocol {
	return "sided"
}

func (this sidedProtocol) Grammar() protocol.Grammar {
	return protocol.Grammar{
		Provides: descriptor.Object(descriptor.Fields{"graph": descriptor.String()}),
		Consumes: descriptor.Object(descriptor.Fields{"query": descriptor.String()}),
	}
}

func (this sidedProtocol) ToDeclarations(protocol.Block) protocol.BlockDeclarations {
	return protocol.BlockDeclarations{}
}

func TestGrammar_TwoRegistriesInOneProcess_EachValidatesSchemasWithItsOwnVocabulary(t *testing.T) {
	withScalar := protocol.NewRegistry([]protocol.Protocol{scalarProtocol{namedProtocol{"graphql"}}, restcontracts.New()})
	withoutScalar := protocol.NewRegistry([]protocol.Protocol{restcontracts.New()})
	document := map[string]any{"schemas": map[string]any{"DateTime": map[string]any{"type": "scalar", "name": "DateTime"}}}

	assert.Empty(t, descriptor.Validate(withScalar.Grammar(), document, "api.yaml"))
	assert.Equal(t, []violation.Violation{
		{
			ErrorCode: "key.unknown",
			Path:      "schemas;DateTime;name",
			Source:    "api.yaml",
			Details:   map[string]any{"key": "name"},
		},
		{
			ErrorCode: "schema.invalid_type",
			Path:      "schemas;DateTime;type",
			Source:    "api.yaml",
			Details: map[string]any{
				"value":   "scalar",
				"allowed": "object, array, map, string, integer, float, number, boolean, enum, union",
			},
		},
	}, descriptor.Validate(withoutScalar.Grammar(), document, "api.yaml"))
}

func TestGrammar_AProtocolAddingAKeyToACoreType_KeepsTheKeysAndRulesOfTheCore(t *testing.T) {
	grammar := protocol.NewRegistry([]protocol.Protocol{labeledUnionProtocol{namedProtocol{"graphql"}}}).Grammar()
	labeled := map[string]any{"schemas": map[string]any{
		"Node": map[string]any{"type": "union", "label": "node", "members": map[string]any{"User": map[string]any{"ref": "User"}}},
		"User": map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}},
	}}
	withoutMembers := map[string]any{"schemas": map[string]any{"Node": map[string]any{"type": "union", "label": "node"}}}

	assert.Empty(t, descriptor.Validate(grammar, labeled, "api.yaml"))
	assert.Equal(t, []violation.Violation{
		{ErrorCode: "members.required", Path: "schemas;Node", Source: "api.yaml", Details: map[string]any{"type": "union"}},
	}, descriptor.Validate(grammar, withoutMembers, "api.yaml"))
}

func TestNewRegistry_TwoProtocolsAddingTheSameKeyToASchemaType_Panics(t *testing.T) {
	assert.PanicsWithError(t, `schema type "scalar" key "name" is defined twice`, func() {
		protocol.NewRegistry([]protocol.Protocol{scalarProtocol{namedProtocol{"graphql"}}, scalarProtocol{namedProtocol{"grpc"}}})
	})
}

func TestRules_JoinsTheRulesOfEachProtocolInProtocolOrder(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{
		ruledProtocol{namedProtocol{"rest"}, []string{"rest.first", "rest.second"}},
		namedProtocol{"grpc"},
		ruledProtocol{namedProtocol{"graphql"}, []string{"graphql.only"}},
	})

	var codes []string
	for _, rule := range registry.Rules() {
		codes = append(codes, rule(protocol.Declarations{})[0].ErrorCode)
	}

	assert.Equal(t, []string{"graphql.only", "rest.first", "rest.second"}, codes)
}

func TestMergeExtensions_HandsEachProtocolItsPiecesInFileOrder(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{joiningProtocol{namedProtocol{"graphql"}}, restcontracts.New()})

	extensions, violations := registry.MergeExtensions(map[string][]protocol.ExtensionPiece{
		"graphql": {{Source: "b.yaml", Value: "second"}, {Source: "a.graphqls", Value: "first"}},
	})

	assert.Equal(t, map[string]any{"graphql": "a.graphqls=first,b.yaml=second"}, extensions)
	assert.Equal(t, []violation.Violation{{ErrorCode: "extension.joined", Source: "a.graphqls"}}, violations)
}

func TestMergeExtensions_NoPieces_NoExtensions(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{joiningProtocol{namedProtocol{"graphql"}}})

	extensions, violations := registry.MergeExtensions(nil)

	assert.Nil(t, extensions)
	assert.Nil(t, violations)
}

func TestMergeExtensions_PiecesOfAProtocolThatMergesNone_Panics(t *testing.T) {
	registry := protocol.NewRegistry([]protocol.Protocol{namedProtocol{"grpc"}})

	assert.PanicsWithError(t, `protocol "grpc" declares extension pieces but merges none`, func() {
		registry.MergeExtensions(map[string][]protocol.ExtensionPiece{"grpc": {{Source: "api.yaml", Value: "x"}}})
	})
}
