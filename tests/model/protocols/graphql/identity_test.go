package graphql_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bidirekt/broker/internal/model"
	graphqlmodel "github.com/bidirekt/broker/internal/model/protocols/graphql"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
)

func TestIdentity_Returns_NamesOperationFieldAndInteraction(t *testing.T) {
	identity := graphqlmodel.Identity{Operation: "query", Field: "pet", Interaction: "returns"}

	assert.Equal(t, []string{"query", "pet", "returns"}, identity.HashParts())
	assert.Equal(t, "query pet", identity.Describe())
	assert.Equal(t, map[string]string{"operation": "query", "field": "pet", "interaction": "returns"}, identity.Location())
}

func TestIdentity_Arguments_NamesOperationFieldAndInteraction(t *testing.T) {
	identity := graphqlmodel.Identity{Operation: "mutation", Field: "adoptPet", Interaction: "arguments"}

	assert.Equal(t, []string{"mutation", "adoptPet", "arguments"}, identity.HashParts())
	assert.Equal(t, "mutation adoptPet", identity.Describe())
	assert.Equal(t, map[string]string{"operation": "mutation", "field": "adoptPet", "interaction": "arguments"}, identity.Location())
}

func TestIdentity_NeverHashesLikeARESTResource(t *testing.T) {
	graphql := model.UploadedResource{Resource: model.Resource{
		ParticipantName: "pets_api",
		Direction:       model.Provides,
		Protocol:        graphqlmodel.Protocol,
		Identity:        graphqlmodel.Identity{Operation: "query", Field: "pet", Interaction: "returns"},
	}}
	rest := model.UploadedResource{Resource: model.Resource{
		ParticipantName: "pets_api",
		Direction:       model.Provides,
		Protocol:        restmodel.Protocol,
		Identity:        restmodel.Identity{Endpoint: "query", Method: "pet", Status: "returns"},
	}}

	assert.NotEqual(t, rest.ProviderHash(), graphql.ProviderHash())
}
