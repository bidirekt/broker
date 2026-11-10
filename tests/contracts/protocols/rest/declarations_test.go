package rest_test

import (
	"testing"

	"github.com/guregu/null"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	restcontracts "github.com/bidirekt/broker/internal/contracts/protocols/rest"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

func consumerBlock(document contract.Document) protocol.Block {
	return protocol.Block{
		Source:           "web.yaml",
		Path:             contract.NewResourcePath("consumes;pets-service;rest"),
		Document:         document,
		Direction:        model.Consumes,
		ConsumedProvider: null.StringFrom("pets-service"),
		Protocol:         restmodel.Protocol,
	}
}

func providerBlock(document contract.Document) protocol.Block {
	return protocol.Block{
		Source:    "api.yaml",
		Path:      contract.NewResourcePath("provides;rest"),
		Document:  document,
		Direction: model.Provides,
		Protocol:  restmodel.Protocol,
	}
}

func TestToResourceDeclarations_ConsumerRequest_IsAnInput(t *testing.T) {
	block := consumerBlock(contract.Document{"/pets": map[string]any{"post": map[string]any{"request": "NewPet"}}})

	declarations := restcontracts.New().ToResourceDeclarations(block)

	require.Len(t, declarations, 1)
	resource := declarations[0].Resource
	assert.Equal(t, "consumes;pets-service;rest;/pets;post;request", declarations[0].Path.String())
	assert.Equal(t, model.Consumes, resource.Direction)
	assert.Equal(t, restmodel.Protocol, resource.Protocol)
	assert.Equal(t, model.Input, resource.Body)
	assert.Equal(t, "pets-service", resource.ConsumedProvider.String)
	assert.Equal(t, restmodel.Identity{Endpoint: "/pets", Method: "post"}, resource.Identity)
}

func TestToResourceDeclarations_ConsumerResponse_IsAnOutputAndCarriesTheBody(t *testing.T) {
	block := consumerBlock(contract.Document{"/pets": map[string]any{"get": map[string]any{"responses": map[string]any{"200": "PetList"}}}})

	declarations := restcontracts.New().ToResourceDeclarations(block)

	require.Len(t, declarations, 1)
	resource := declarations[0].Resource
	assert.Equal(t, "consumes;pets-service;rest;/pets;get;responses;200", declarations[0].Path.String())
	assert.Equal(t, model.Consumes, resource.Direction)
	assert.Equal(t, model.Output, resource.Body)
	assert.Equal(t, "pets-service", resource.ConsumedProvider.String)
	assert.Equal(t, restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"}, resource.Identity)
	assert.Equal(t, []model.ResourceBody{{Source: "web.yaml", SchemaName: null.StringFrom("PetList")}}, resource.Bodies)
	assert.Equal(t, "PetList", declarations[0].SchemaName)
}

func TestToResourceDeclarations_ProviderRequest_IsAnInput(t *testing.T) {
	block := providerBlock(contract.Document{"/pets": map[string]any{"post": map[string]any{"request": "NewPet"}}})

	declarations := restcontracts.New().ToResourceDeclarations(block)

	require.Len(t, declarations, 1)
	resource := declarations[0].Resource
	assert.Equal(t, "provides;rest;/pets;post;request", declarations[0].Path.String())
	assert.Equal(t, model.Provides, resource.Direction)
	assert.Equal(t, model.Input, resource.Body)
	assert.Empty(t, resource.ConsumedProvider)
	assert.Equal(t, restmodel.Identity{Endpoint: "/pets", Method: "post"}, resource.Identity)
}

func TestToResourceDeclarations_ProviderResponse_IsAnOutput(t *testing.T) {
	block := providerBlock(contract.Document{"/pets": map[string]any{"get": map[string]any{"responses": map[string]any{"200": "PetList"}}}})

	declarations := restcontracts.New().ToResourceDeclarations(block)

	require.Len(t, declarations, 1)
	resource := declarations[0].Resource
	assert.Equal(t, "provides;rest;/pets;get;responses;200", declarations[0].Path.String())
	assert.Equal(t, model.Provides, resource.Direction)
	assert.Equal(t, model.Output, resource.Body)
	assert.Equal(t, restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"}, resource.Identity)
}

func TestToResourceDeclarations_ABodyWithoutSchema_IsDeclaredWithoutSchema(t *testing.T) {
	block := providerBlock(contract.Document{"/pets/*": map[string]any{"delete": map[string]any{"responses": map[string]any{"204": nil}}}})

	declarations := restcontracts.New().ToResourceDeclarations(block)

	require.Len(t, declarations, 1)
	assert.True(t, declarations[0].WithoutSchema)
	assert.Equal(t, []model.ResourceBody{{Source: "api.yaml"}}, declarations[0].Resource.Bodies)
}

func TestToResourceDeclarations_NormalizesTheEndpointInPathAndIdentity(t *testing.T) {
	block := providerBlock(contract.Document{"/pets/": map[string]any{"get": map[string]any{"responses": map[string]any{"200": "PetList"}}}})

	declarations := restcontracts.New().ToResourceDeclarations(block)

	require.Len(t, declarations, 1)
	assert.Equal(t, "provides;rest;/pets;get;responses;200", declarations[0].Path.String())
	assert.Equal(t, "/pets", declarations[0].Resource.Identity.(restmodel.Identity).Endpoint)
}

func TestToResourceDeclarations_AnAbsentBlock_DeclaresNothing(t *testing.T) {
	assert.Empty(t, restcontracts.New().ToResourceDeclarations(providerBlock(nil)))
}
