package graphql_test

import (
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	graphqlcontracts "github.com/bidirekt/broker/internal/contracts/protocols/graphql"
	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
)

func consumedGraphQLBlock(document contract.Document) protocol.Block {
	return protocol.Block{
		Source:           "web.yaml",
		Path:             contract.NewResourcePath("consumes;petstore_api;graphql"),
		Document:         document,
		Direction:        model.Consumes,
		ConsumedProvider: null.StringFrom("petstore_api"),
		Protocol:         graphqlmodel.Protocol,
	}
}

func TestToDeclarations_AField_IsItsArgumentsInputAndItsReturnsOutput(t *testing.T) {
	block := consumedGraphQLBlock(contract.Document{"query": map[string]any{"pet": map[string]any{"arguments": "PetArgs", "returns": "MaybePet"}}})

	declarations := graphqlcontracts.New().ToDeclarations(block)

	assert.Empty(t, declarations.Schemas)
	assert.Equal(t, []protocol.ResourceDeclaration{
		{
			Source:     "web.yaml",
			Path:       contract.NewResourcePath("consumes;petstore_api;graphql;query;pet;arguments"),
			Namespace:  model.YAMLNamespace,
			SchemaName: "PetArgs",
			Resource: model.UploadedResource{Resource: model.Resource{
				Direction:        model.Consumes,
				Protocol:         graphqlmodel.Protocol,
				Body:             model.Input,
				Identity:         graphqlmodel.Identity{Operation: "query", Field: "pet", Interaction: "arguments"},
				ConsumedProvider: null.StringFrom("petstore_api"),
				Bodies:           []model.ResourceBody{{Source: "web.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("PetArgs")}},
			}},
		},
		{
			Source:     "web.yaml",
			Path:       contract.NewResourcePath("consumes;petstore_api;graphql;query;pet;returns"),
			Namespace:  model.YAMLNamespace,
			SchemaName: "MaybePet",
			Resource: model.UploadedResource{Resource: model.Resource{
				Direction:        model.Consumes,
				Protocol:         graphqlmodel.Protocol,
				Body:             model.Output,
				Identity:         graphqlmodel.Identity{Operation: "query", Field: "pet", Interaction: "returns"},
				ConsumedProvider: null.StringFrom("petstore_api"),
				Bodies:           []model.ResourceBody{{Source: "web.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("MaybePet")}},
			}},
		},
	}, declarations.Resources)
}

func TestToDeclarations_AFieldWithoutArguments_SendsTheGeneratedEmptyArguments(t *testing.T) {
	block := consumedGraphQLBlock(contract.Document{"query": map[string]any{"pets": map[string]any{"returns": "PetList"}}})

	declarations := graphqlcontracts.New().ToDeclarations(block)

	require.Len(t, declarations.Resources, 2)
	arguments := declarations.Resources[0]
	assert.Equal(t, "consumes;petstore_api;graphql;query;pets;arguments", arguments.Path.String())
	assert.Equal(t, "graphql_generated", arguments.Namespace)
	assert.Equal(t, "EmptyArguments", arguments.SchemaName)
	assert.False(t, arguments.WithoutSchema)
	assert.Equal(t, model.Input, arguments.Resource.Body)
	assert.Equal(t, graphqlmodel.Identity{Operation: "query", Field: "pets", Interaction: "arguments"}, arguments.Resource.Identity)
	assert.Equal(t, []model.ResourceBody{{Source: "web.yaml", Namespace: "graphql_generated", SchemaName: null.StringFrom("EmptyArguments")}}, arguments.Resource.Bodies)
	assert.Equal(t, []protocol.SchemaDeclaration{{
		Source:    "web.yaml",
		Namespace: "graphql_generated",
		Name:      "EmptyArguments",
		Schema:    contract.Schema{Type: model.ObjectType},
	}}, declarations.Schemas)
}

func TestToDeclarations_SeveralFieldsWithoutArguments_GenerateOneEmptyArgumentsPerBlock(t *testing.T) {
	block := consumedGraphQLBlock(contract.Document{
		"query":    map[string]any{"pets": map[string]any{"returns": "PetList"}, "owners": map[string]any{"arguments": nil, "returns": "OwnerList"}},
		"mutation": map[string]any{"adoptAny": map[string]any{"returns": "Pet"}},
	})

	declarations := graphqlcontracts.New().ToDeclarations(block)

	require.Len(t, declarations.Schemas, 1)
	assert.Equal(t, "EmptyArguments", declarations.Schemas[0].Name)

	var described []string
	for _, declaration := range declarations.Resources {
		described = append(described, declaration.Path.String()+" "+declaration.Namespace+" "+declaration.SchemaName)
	}

	assert.Equal(t, []string{
		"consumes;petstore_api;graphql;query;owners;arguments graphql_generated EmptyArguments",
		"consumes;petstore_api;graphql;query;owners;returns yaml OwnerList",
		"consumes;petstore_api;graphql;query;pets;arguments graphql_generated EmptyArguments",
		"consumes;petstore_api;graphql;query;pets;returns yaml PetList",
		"consumes;petstore_api;graphql;mutation;adoptAny;arguments graphql_generated EmptyArguments",
		"consumes;petstore_api;graphql;mutation;adoptAny;returns yaml Pet",
	}, described)
}

func TestToDeclarations_TheHashParts_AreTheProvidedOnes(t *testing.T) {
	block := consumedGraphQLBlock(contract.Document{"mutation": map[string]any{"adoptPet": map[string]any{"arguments": "AdoptArgs", "returns": "Pet"}}})

	declarations := graphqlcontracts.New().ToDeclarations(block)

	require.Len(t, declarations.Resources, 2)
	assert.Equal(t, []string{"mutation", "adoptPet", "arguments"}, declarations.Resources[0].Resource.Identity.HashParts())
	assert.Equal(t, []string{"mutation", "adoptPet", "returns"}, declarations.Resources[1].Resource.Identity.HashParts())
}

func TestToDeclarations_AProvidesBlock_DeclaresNothing(t *testing.T) {
	block := protocol.Block{
		Source:    "api.yaml",
		Path:      contract.NewResourcePath("provides;graphql"),
		Document:  contract.Document{"graph": "petstore_graph"},
		Direction: model.Provides,
		Protocol:  graphqlmodel.Protocol,
	}

	assert.Empty(t, graphqlcontracts.New().ToDeclarations(block))
}
