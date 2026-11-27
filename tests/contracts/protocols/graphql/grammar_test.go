package graphql_test

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/contracts/violation"
)

func grammarViolations(t *testing.T, source string) []violation.Violation {
	t.Helper()

	var document any
	require.NoError(t, yaml.Unmarshal([]byte(source), &document))

	grammar := protocol.NewRegistry([]protocol.Protocol{graphqlcontracts.New()}).Grammar()

	return descriptor.Validate(grammar, document, "web.yaml")
}

func TestGrammar_AConsumerOfQueriesAndMutations_IsValid(t *testing.T) {
	violations := grammarViolations(t, `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs, returns: MaybePet}
        pets: {returns: PetList}
      mutation:
        adoptPet: {arguments: AdoptArgs, returns: Pet}
`)

	assert.Empty(t, violations)
}

func TestGrammar_AnEmptyBlock_IsValidOnBothSides(t *testing.T) {
	violations := grammarViolations(t, "provides:\n  graphql: {}\nconsumes:\n  petstore_api:\n    graphql: ~\n")

	assert.Empty(t, violations)
}

func TestGrammar_TheGraphUnderProvides_IsAccepted(t *testing.T) {
	violations := grammarViolations(t, "provides:\n  graphql:\n    graph: petstore_graph\n")

	assert.Empty(t, violations)
}

func TestGrammar_KeysOfTheOtherSideAndSubscriptions_AreUnknown(t *testing.T) {
	violations := grammarViolations(t, `provides:
  graphql:
    query:
      pet: {returns: Pet}
    mutation:
      adoptPet: {returns: Pet}
consumes:
  petstore_api:
    graphql:
      graph: petstore_graph
      subscription:
        petAdopted: {returns: Pet}
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "key.unknown", Path: "consumes;petstore_api;graphql;graph", Source: "web.yaml", Details: map[string]any{"key": "graph"}},
		{ErrorCode: "key.unknown", Path: "consumes;petstore_api;graphql;subscription", Source: "web.yaml", Details: map[string]any{"key": "subscription"}},
		{ErrorCode: "key.unknown", Path: "provides;graphql;mutation", Source: "web.yaml", Details: map[string]any{"key": "mutation"}},
		{ErrorCode: "key.unknown", Path: "provides;graphql;query", Source: "web.yaml", Details: map[string]any{"key": "query"}},
	}, violations)
}

func TestGrammar_AFieldWithoutReturns_IsRefusedAtTheField(t *testing.T) {
	violations := grammarViolations(t, `consumes:
  petstore_api:
    graphql:
      query:
        pet: {arguments: PetArgs}
        pets: ~
      mutation:
        adoptPet: {returns: ~}
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "returns.required", Path: "consumes;petstore_api;graphql;mutation;adoptPet", Source: "web.yaml", Details: map[string]any{}},
		{ErrorCode: "returns.required", Path: "consumes;petstore_api;graphql;query;pet", Source: "web.yaml", Details: map[string]any{}},
		{ErrorCode: "returns.required", Path: "consumes;petstore_api;graphql;query;pets", Source: "web.yaml", Details: map[string]any{}},
	}, violations)
}

func TestGrammar_AFieldNameOutsideTheGraphQLName_IsRefusedAtTheKey(t *testing.T) {
	violations := grammarViolations(t, `consumes:
  petstore_api:
    graphql:
      query:
        "pets(first: 10)": {returns: PetList}
        1pet: {returns: Pet}
      mutation:
        adopt-pet: {returns: Pet}
`)

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "operation.name_syntax", Path: "consumes;petstore_api;graphql;mutation;adopt-pet", Source: "web.yaml", Details: map[string]any{"name": "adopt-pet"}},
		{ErrorCode: "operation.name_syntax", Path: "consumes;petstore_api;graphql;query;1pet", Source: "web.yaml", Details: map[string]any{"name": "1pet"}},
		{ErrorCode: "operation.name_syntax", Path: "consumes;petstore_api;graphql;query;pets(first: 10)", Source: "web.yaml", Details: map[string]any{"name": "pets(first: 10)"}},
	}, violations)
}

func TestGrammar_GraphQLNames_AreAccepted(t *testing.T) {
	violations := grammarViolations(t, `consumes:
  petstore_api:
    graphql:
      query:
        _pet: {returns: Pet}
        Pet2_list: {returns: Pet}
`)

	assert.Empty(t, violations)
}

func TestGrammar_AGraphOutsideTheParticipantNameFormat_IsRefusedAtTheGraph(t *testing.T) {
	violations := grammarViolations(t, "provides:\n  graphql:\n    graph: Petstore-Graph\n")

	assert.Equal(t, []violation.Violation{
		{ErrorCode: "graph.name_syntax", Path: "provides;graphql;graph", Source: "web.yaml", Details: map[string]any{"graph": "Petstore-Graph", "error": "must be snake_case"}},
	}, violations)
}
